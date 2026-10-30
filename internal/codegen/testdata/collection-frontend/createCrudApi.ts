import {
	createCrudApi as createLibraryCrudApi,
	type CreateCrudApiOptions,
} from "@katren/vue-collection-lib/api/createCrudApi";
import api from "@/api/http";

// Application-owned adapter: retain the library's complete request object.
export const createCrudApi = <TList, TListDTO, TDetail, TDetailDTO, TKey, TNew, TUpd>(
	options: Omit<CreateCrudApiOptions<TList, TListDTO, TDetail, TDetailDTO, TKey, TNew, TUpd>, "api">,
) => createLibraryCrudApi({ ...options, api });
