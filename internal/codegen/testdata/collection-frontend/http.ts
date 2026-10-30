import type { CrudHttpClient } from "@katren/vue-collection-lib/api/createCrudApi";

export const requests: { endpoint: string; params?: Record<string, string> }[] = [];

const api: CrudHttpClient = {
	get: async <T>(endpoint: string, params?: Record<string, string>): Promise<T> => {
		requests.push({ endpoint, params });
		return { rows: [{ code: "sku-2", name: "Two" }], agg: { tot_count: 1 } } as T;
	},
	post: async <T>(): Promise<T> => { throw new Error("unexpected post"); },
	patch: async <T>(): Promise<T> => { throw new Error("unexpected patch"); },
	delete: async <T>(): Promise<T> => { throw new Error("unexpected delete"); },
};

export default api;
