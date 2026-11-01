/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

/**
 * Anthropic Messages 或 OpenAI Chat Completions 兼容协议；旧配置默认为 Anthropic。
 */
export type GatewayProtocol = typeof GatewayProtocol[keyof typeof GatewayProtocol];


export const GatewayProtocol = {
  anthropic: 'anthropic',
  openai: 'openai',
} as const;
