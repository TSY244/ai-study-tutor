/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { QuestionInputCognitiveLevel } from './questionInputCognitiveLevel';
import type { QuestionInputDifficulty } from './questionInputDifficulty';
import type { QuestionInputOptions } from './questionInputOptions';
import type { QuestionInputType } from './questionInputType';

export interface QuestionInput {
  type: QuestionInputType;
  /**
     * @minLength 1
     * @maxLength 20000
     */
  stem: string;
  /** @nullable */
  options?: QuestionInputOptions;
  correct_answer: unknown;
  /** @nullable */
  explanation?: string | null;
  tested_concepts?: string[];
  difficulty: QuestionInputDifficulty;
  cognitive_level: QuestionInputCognitiveLevel;
  /**
     * @maximum 999999.99
     * @exclusiveMinimum 0
     */
  score: number;
}
