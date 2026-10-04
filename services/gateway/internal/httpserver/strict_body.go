package httpserver

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
)

// maximumBodyNesting bounds how deep a command body may nest; no command has a body anywhere near it.
const maximumBodyNesting = 32

var (
	rawMessageType  = reflect.TypeOf(json.RawMessage(nil))
	unmarshalerType = reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
)

// rejectDuplicateKeys walks the JSON tokens of a command body: no object, at any depth, may repeat
// a key, and nesting stays shallow. encoding/json keeps the last of two equal keys silently, which
// would let `{"decision":"reject","decision":"approve"}` read differently in two parsers. (The same
// walk, with a null rule, is in policy/canonical.go for tool arguments; httpserver cannot import
// policy.)
func rejectDuplicateKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	return walkBodyValue(decoder, 0)
}

func walkBodyValue(decoder *json.Decoder, depth int) error {
	if depth > maximumBodyNesting {
		return errMalformedBody
	}
	token, err := decoder.Token()
	if err != nil {
		return errMalformedBody
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seenKeys := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, isString := keyToken.(string)
			if err != nil || !isString || seenKeys[key] {
				return errMalformedBody
			}
			seenKeys[key] = true
			if err := walkBodyValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := walkBodyValue(decoder, depth+1); err != nil {
				return err
			}
		}
	}
	if _, err := decoder.Token(); err != nil { // the closing brace or bracket
		return errMalformedBody
	}
	return nil
}

// requireExactKeys checks every object key of the body against the json tags of the target type,
// byte for byte. encoding/json matches field names case-insensitively, so `{"DECISION":"approve"}`
// would otherwise decode as the `decision` field. Values that decode themselves (json.RawMessage and
// other Unmarshalers) and maps are not looked into; their own consumers are strict.
func requireExactKeys(body []byte, target any) error {
	var tree any
	if err := json.Unmarshal(body, &tree); err != nil {
		return errMalformedBody
	}
	return matchKeys(tree, reflect.TypeOf(target))
}

func matchKeys(node any, valueType reflect.Type) error {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if valueType == rawMessageType || reflect.PointerTo(valueType).Implements(unmarshalerType) {
		return nil
	}
	switch valueType.Kind() {
	case reflect.Struct:
		object, isObject := node.(map[string]any)
		if !isObject {
			return nil // a type mismatch is the decoder's to refuse
		}
		fields := exactFieldTypes(valueType)
		for key, value := range object {
			fieldType, known := fields[key]
			if !known {
				return errMalformedBody
			}
			if err := matchKeys(value, fieldType); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		list, isList := node.([]any)
		if !isList {
			return nil
		}
		for _, element := range list {
			if err := matchKeys(element, valueType.Elem()); err != nil {
				return err
			}
		}
	}
	return nil
}

// exactFieldTypes maps the JSON name of every exported field of a struct to its type, flattening
// untagged embedded structs as encoding/json does.
func exactFieldTypes(structType reflect.Type) map[string]reflect.Type {
	fields := map[string]reflect.Type{}
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
		tag, tagged := field.Tag.Lookup("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" && !strings.Contains(tag, ",") {
			continue
		}
		if field.Anonymous && name == "" {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				for embeddedName, embeddedType := range exactFieldTypes(embedded) {
					fields[embeddedName] = embeddedType
				}
				continue
			}
		}
		if !field.IsExported() {
			continue
		}
		if !tagged || name == "" {
			name = field.Name
		}
		fields[name] = field.Type
	}
	return fields
}
