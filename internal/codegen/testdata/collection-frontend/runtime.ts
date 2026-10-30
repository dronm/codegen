import type { CollectionParams } from "@katren/vue-collection-lib/types/collection";
import { stockItemApi } from "@/api/stockItem.gen";
import { requests } from "@/api/http";
import { stockItemCollection } from "@/collections/stockItem.gen";

const main = async (): Promise<void> => {
	const params: CollectionParams = {
		filter: [{ f: { name: { o: "ne", v: "" } } }],
		where: { or: [{ f: "code", o: "g", v: "sku-1" }, { and: [{ f: "code", o: "e", v: "sku-1" }] }] },
		sorter: [{ f: "code", d: "a" }],
		from: 0,
		count: 101,
	};
	const response = await stockItemApi.list(params);
	if (requests[0]?.endpoint !== "/stock-items" || requests[0]?.params?.params !== JSON.stringify(params)) {
		throw new Error("generated API lost complete CollectionParams, including where");
	}
	if (response.rows[0]?.code !== "sku-2" || stockItemCollection.infiniteScroll?.keyFields?.[0] !== "code") {
		throw new Error("projection conversion or generated cursor key was lost");
	}
};

void main();
