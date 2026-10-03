import { PipeTransform, ArgumentMetadata, BadRequestException, Injectable } from "@nestjs/common";
import { ZodSchema } from "zod";

@Injectable()
export class ZodValidationPipe implements PipeTransform {
  constructor(private schema: ZodSchema) {}

  transform(value: unknown, metadata: ArgumentMetadata) {
    try {
      if (metadata.type === "body") {
        const parsedValue = this.schema.parse(value);
        return parsedValue;
      }
      return value;
    } catch {
      throw new BadRequestException("Validation failed");
    }
  }
}
