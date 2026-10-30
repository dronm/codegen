import * as v from "valibot";

export type TranslateFn = (key: string) => string;
export const defaultTranslate: TranslateFn = (key) => key;
export const createCommonSchemas = (_t: TranslateFn) => ({
	RequiredStringSchema: v.string(),
});
