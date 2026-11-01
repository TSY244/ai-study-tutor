/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export interface CourseCreate {
  /**
     * 可选；省略或空白时先显示待 AI 命名，首次成功生成学习总结后自动命名。
     * @maxLength 200
     */
  title?: string;
  /** @maxLength 5000 */
  description?: string;
}
