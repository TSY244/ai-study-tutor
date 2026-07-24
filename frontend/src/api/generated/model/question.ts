/**
 * GENERATED FILE - DO NOT EDIT
 * Source: api/openapi.yaml (发布契约)
 * Regenerate: npm run api:gen
 */
import type { QuestionCognitiveLevel } from './questionCognitiveLevel';
import type { QuestionDifficulty } from './questionDifficulty';
import type { QuestionOptions } from './questionOptions';
import type { QuestionType } from './questionType';

/**
 * 发布后不可变、学生可见的题目版本；标准答案和解析不在本对象中返回。
 */
export interface Question {
  id: string;
  type: QuestionType;
  stem: string;
  /** @nullable */
  options: QuestionOptions;
  tested_concepts: string[];
  difficulty: QuestionDifficulty;
  cognitive_level: QuestionCognitiveLevel;
  /** @exclusiveMinimum 0 */
  score: number;
}
