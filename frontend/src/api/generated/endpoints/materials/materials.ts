/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type {
  ListMaterialsParams,
  MaterialDetail,
  MaterialJobAccepted,
  MaterialList,
  MaterialUpdate,
  TextMaterialCreate,
  UploadMaterialBody,
  UrlMaterialCreate
} from '../../model';

import { apiFetch } from '../../../http';

export const getCreateUrlMaterialUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/materials/url`
}

/**
 * @summary 添加 URL 素材并创建导入任务
 */
export const createUrlMaterial = async (lessonId: string,
    urlMaterialCreate: UrlMaterialCreate, options?: RequestInit): Promise<MaterialJobAccepted> => {

  return apiFetch<MaterialJobAccepted>(getCreateUrlMaterialUrl(lessonId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(urlMaterialCreate)
  }
);}


export const getCreateTextMaterialUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/materials/text`
}

/**
 * @summary 添加文本素材并创建导入任务
 */
export const createTextMaterial = async (lessonId: string,
    textMaterialCreate: TextMaterialCreate, options?: RequestInit): Promise<MaterialJobAccepted> => {

  return apiFetch<MaterialJobAccepted>(getCreateTextMaterialUrl(lessonId),
  {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(textMaterialCreate)
  }
);}


export const getUploadMaterialUrl = (lessonId: string,) => {




  return `/api/v1/lessons/${lessonId}/materials/upload`
}

/**
 * @summary 上传文件素材并创建导入任务
 */
export const uploadMaterial = async (lessonId: string,
    uploadMaterialBody: UploadMaterialBody, options?: RequestInit): Promise<MaterialJobAccepted> => {
    const formData = new FormData();
formData.append(`file`, uploadMaterialBody.file);
if(uploadMaterialBody.preferred_language !== undefined) {
 formData.append(`preferred_language`, uploadMaterialBody.preferred_language);
 }
if(uploadMaterialBody.transcription_mode !== undefined) {
 formData.append(`transcription_mode`, uploadMaterialBody.transcription_mode);
 }

  return apiFetch<MaterialJobAccepted>(getUploadMaterialUrl(lessonId),
  {
    ...options,
    method: 'POST'
    ,
    body: formData
  }
);}


export const getListMaterialsUrl = (lessonId: string,
    params?: ListMaterialsParams,) => {
  const normalizedParams = new URLSearchParams();

  Object.entries(params || {}).forEach(([key, value]) => {

    if (value !== undefined) {
      normalizedParams.append(key, value === null ? 'null' : String(value))
    }
  });

  const stringifiedParams = normalizedParams.toString();

  return stringifiedParams.length > 0 ? `/api/v1/lessons/${lessonId}/materials?${stringifiedParams}` : `/api/v1/lessons/${lessonId}/materials`
}

/**
 * @summary 查询课时素材
 */
export const listMaterials = async (lessonId: string,
    params?: ListMaterialsParams, options?: RequestInit): Promise<MaterialList> => {

  return apiFetch<MaterialList>(getListMaterialsUrl(lessonId,params),
  {
    ...options,
    method: 'GET'


  }
);}


export const getDeleteMaterialUrl = (materialId: string,) => {




  return `/api/v1/materials/${materialId}`
}

/**
 * @summary 删除未被活动任务使用的素材
 */
export const deleteMaterial = async (materialId: string, options?: RequestInit): Promise<void> => {

  return apiFetch<void>(getDeleteMaterialUrl(materialId),
  {
    ...options,
    method: 'DELETE'


  }
);}


export const getGetMaterialUrl = (materialId: string,) => {




  return `/api/v1/materials/${materialId}`
}

/**
 * @summary 读取素材编辑内容
 */
export const getMaterial = async (materialId: string, options?: RequestInit): Promise<MaterialDetail> => {

  return apiFetch<MaterialDetail>(getGetMaterialUrl(materialId),
  {
    ...options,
    method: 'GET'


  }
);}


export const getUpdateMaterialUrl = (materialId: string,) => {




  return `/api/v1/materials/${materialId}`
}

/**
 * 至少提供一个修改字段，携带 version。素材 kind 不可变；文本可改 text，链接可改 source_url。上传文件替换使用原上传接口，不能伪造 file_object_id。返回重新导入任务，旧文档保留直到新结果发布。
 * @summary 修改素材并重新导入
 */
export const updateMaterial = async (materialId: string,
    materialUpdate: MaterialUpdate, options?: RequestInit): Promise<MaterialJobAccepted> => {

  return apiFetch<MaterialJobAccepted>(getUpdateMaterialUrl(materialId),
  {
    ...options,
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    body: JSON.stringify(materialUpdate)
  }
);}


