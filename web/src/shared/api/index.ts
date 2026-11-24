
import { aiApi } from './aiApi';
import { appApi } from './appApi';
import { canvasApi } from './canvasApi';
import { gitApi } from './gitApi';
import { harnessApi } from './harnessApi';
import { itemApi } from './itemApi';
import { knowledgeApi } from './knowledgeApi';
import { systemApi } from './systemApi';
import { verificationApi } from './verificationApi';
import { workspaceApi } from './workspaceApi';
import { workspaceFilesApi } from './workspaceFilesApi';

export { ApiError, ApiResponseError, apiURL, isExtensionSurface, localAPIOrigin } from './http';
export { editableStatusOrder, statusLabels, statusOrder } from './statusLabels';

export const api = {
	...canvasApi,
	...knowledgeApi,
	...aiApi,
	...appApi,
	...workspaceApi,
	...workspaceFilesApi,
	...verificationApi,
	...systemApi,
	...itemApi,
	...gitApi,
	...harnessApi,
};

