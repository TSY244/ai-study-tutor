/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */

export type QuestionType = typeof QuestionType[keyof typeof QuestionType];


export const QuestionType = {
  single_choice: 'single_choice',
  multiple_choice: 'multiple_choice',
  true_false: 'true_false',
  short_answer: 'short_answer',
  case_analysis: 'case_analysis',
} as const;
