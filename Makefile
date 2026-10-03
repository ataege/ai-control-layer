# Thin aliases for judges who expect make (report 1.2 names these targets). The logic lives in the
# pnpm scripts; make is optional.
.PHONY: verify-controls reset-demo

verify-controls:
	pnpm verify:controls

reset-demo:
	pnpm reset:demo
