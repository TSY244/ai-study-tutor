import { parseArgs } from 'node:util';
import { pathToFileURL } from 'node:url';
import { InMemoryCredentialStore, InMemoryModelsStore } from '@earendil-works/pi-ai';
import {
  createAgentSession, createExtensionRuntime, ModelRuntime, SessionManager, SettingsManager,
} from '@earendil-works/pi-coding-agent';

async function safeAPIErrorCode(response) {
  const reader = response.clone().body?.getReader();
  if (!reader) return '';
  try {
    const decoder = new TextDecoder();
    let raw = '';
    let size = 0;
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      size += value.length;
      if (size > 65536) return '';
      raw += decoder.decode(value, { stream: true });
    }
    const error = JSON.parse(raw + decoder.decode())?.error;
    const quotaCodes = ['insufficient_quota', 'insufficient_user_quota', 'insufficient_token_quota', 'quota_exhausted', 'billing_hard_limit_reached'];
    if (quotaCodes.includes(error?.code) || quotaCodes.includes(error?.type)) return 'claude_quota_exhausted';
  } catch {} finally {
    void reader.cancel().catch(() => {});
  }
  return '';
}

// One job per process. No discovery of extensions, skills, project instructions,
// auth files or saved sessions from the shared Worker HOME.
export async function runAgent({ prompt, tools, env = process.env, onProgress = () => {} }) {
  const started = Date.now();
  const usage = { input_tokens: 0, output_tokens: 0, cache_read_input_tokens: 0, cache_creation_input_tokens: 0 };
  let session;
  let apiStatus = 0;
  let apiErrorCode = '';
  const nativeFetch = globalThis.fetch;
  globalThis.fetch = async (...args) => {
    const response = await nativeFetch(...args);
    if (response.status >= 400) {
      apiStatus = response.status;
      apiErrorCode = await safeAPIErrorCode(response);
    }
    return response;
  };
  try {
    const protocol = env.AI_STUDY_TUTOR_AI_PROTOCOL || 'anthropic';
    if (!['anthropic', 'openai'].includes(protocol)) throw new Error('Invalid protocol');
    const openai = protocol === 'openai';
    const baseUrl = env.ANTHROPIC_BASE_URL || (openai ? 'https://api.openai.com/v1' : 'https://api.anthropic.com');
    if (!env.ANTHROPIC_AUTH_TOKEN || !env.ANTHROPIC_MODEL || !prompt) throw new Error('Missing runtime configuration');
    if (!tools.length || tools.some((tool) => !['read', 'write', 'bash'].includes(tool))) throw new Error('Invalid tools');
    const modelRuntime = await ModelRuntime.create({
      credentials: new InMemoryCredentialStore(), modelsStore: new InMemoryModelsStore(),
      modelsPath: null, refreshOnCreate: false, allowModelNetwork: false,
    });
    modelRuntime.registerProvider('study-tutor', {
      baseUrl,
      api: openai ? 'openai-completions' : 'anthropic-messages',
      models: [{
        id: env.ANTHROPIC_MODEL, name: env.ANTHROPIC_MODEL,
        reasoning: false, input: ['text', 'image'], contextWindow: 128000, maxTokens: 8192,
        ...(openai ? { compat: {
          supportsStore: false, supportsDeveloperRole: false, supportsReasoningEffort: false, supportsStrictMode: false,
          maxTokensField: new URL(baseUrl).hostname === 'api.openai.com' ? 'max_completion_tokens' : 'max_tokens',
        } } : {}),
        cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
      }],
    });
    await modelRuntime.setRuntimeApiKey('study-tutor', env.ANTHROPIC_AUTH_TOKEN);
    // Job retries and deadlines are owned by Go. Do not multiply retries inside Pi.
    const streamSimple = modelRuntime.streamSimple.bind(modelRuntime);
    modelRuntime.streamSimple = (model, context, options) => streamSimple(model, context, { ...options, maxRetries: 0 });
    const resourceLoader = {
      getExtensions: () => ({ extensions: [], errors: [], runtime: createExtensionRuntime() }),
      getSkills: () => ({ skills: [], diagnostics: [] }),
      getPrompts: () => ({ prompts: [], diagnostics: [] }),
      getThemes: () => ({ themes: [], diagnostics: [] }),
      getAgentsFiles: () => ({ agentsFiles: [] }),
      getSystemPrompt: () => undefined, getSystemPromptSource: () => undefined,
      getAppendSystemPrompt: () => [], getAppendSystemPromptSources: () => [],
      extendResources: () => {}, reload: async () => {},
    };
    ({ session } = await createAgentSession({
      cwd: process.cwd(), modelRuntime, model: modelRuntime.getModel('study-tutor', env.ANTHROPIC_MODEL),
      thinkingLevel: 'off', tools, resourceLoader,
      sessionManager: SessionManager.inMemory(process.cwd()),
      settingsManager: SettingsManager.inMemory({ compaction: { enabled: false }, retry: { enabled: false } }),
    }));
    let turns = 0;
    let characters = 0;
    let streamed = false;
    let lastProgressAt = 0;
    const progress = (phase, tool = '', force = false) => {
      const now = Date.now();
      if (!force && now - lastProgressAt < 750) return;
      lastProgressAt = now;
      // Only counters and allowlisted states cross the process boundary.
      // Deltas, thinking, tool arguments/results and paths remain private.
      try { onProgress({ type: 'pi_progress', phase, tool, characters, turns: Math.min(turns + 1, 40) }); } catch {}
    };
    session.subscribe((event) => {
      if (event.type === 'message_update') {
        const update = event.assistantMessageEvent;
        if (['text_delta', 'toolcall_delta'].includes(update?.type) && typeof update.delta === 'string') {
          characters += [...update.delta].length;
          progress('generating', '', !streamed);
          streamed = true;
        }
      }
      if (['tool_execution_start', 'tool_execution_end'].includes(event.type) && ['read', 'write', 'bash'].includes(event.toolName)) {
        progress(event.type === 'tool_execution_start' ? 'tool_start' : 'tool_end', event.toolName, true);
      }
      if (event.type !== 'message_end' || event.message.role !== 'assistant') return;
      const u = event.message.usage;
      usage.input_tokens += u?.input || 0;
      usage.output_tokens += u?.output || 0;
      usage.cache_read_input_tokens += u?.cacheRead || 0;
      usage.cache_creation_input_tokens += u?.cacheWrite || 0;
      if (++turns >= 40 && event.message.stopReason === 'toolUse') void session.abort();
    });
    progress('started', '', true);
    await session.prompt(prompt);
    const last = session.messages.filter((message) => message.role === 'assistant').at(-1);
    // prompt() can resolve on a failed/aborted model response. Check the terminal
    // message before accepting files left by earlier tool calls.
    if (!last || last.stopReason !== 'stop') throw new Error('Pi did not finish successfully');
    return {
      type: 'result', subtype: 'success', is_error: false,
      result: session.getLastAssistantText() || '', duration_ms: Date.now() - started,
      session_id: session.sessionId, usage,
    };
  } catch (error) {
    // Never serialize upstream errors: they may echo credentials or source text.
    return {
      type: 'result', subtype: 'error', is_error: true, result: '',
      api_error_status: apiStatus || (Number.isInteger(error?.status) ? error.status : 0),
      api_error_code: apiErrorCode,
      duration_ms: Date.now() - started, session_id: session?.sessionId || '', usage,
    };
  } finally {
    session?.dispose();
    globalThis.fetch = nativeFetch;
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  let result;
  try {
    const { values } = parseArgs({ options: { prompt: { type: 'string' }, tools: { type: 'string' } } });
    result = await runAgent({ prompt: values.prompt, tools: (values.tools || '').split(','),
      onProgress: (event) => process.stderr.write(JSON.stringify(event) + '\n'),
    });
  } catch {
    result = { type: 'result', subtype: 'error', is_error: true, api_error_status: 0, result: '' };
  }
  process.stdout.write(JSON.stringify(result) + '\n');
  process.exitCode = result.is_error ? 1 : 0;
}
