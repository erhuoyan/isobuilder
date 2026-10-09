import { getAuthorization, request, serviceBaseURL } from '../request';

/** enqueue a build for a project */
export function fetchCreateBuild(projectId: string) {
  return request<{ build: Api.IsoBuilder.Build }>({
    url: `/api/projects/${projectId}/builds`,
    method: 'post'
  });
}

/** get builds, newest first */
export function fetchGetBuilds(params: { project_id?: string; limit?: number } = {}) {
  return request<{ builds: Api.IsoBuilder.Build[] }>({ url: '/api/builds', params });
}

/** get one build */
export function fetchGetBuild(id: string) {
  return request<{ build: Api.IsoBuilder.Build }>({ url: `/api/builds/${id}` });
}

/** cancel a running build */
export function fetchCancelBuild(id: string) {
  return request<{ build: Api.IsoBuilder.Build }>({ url: `/api/builds/${id}/cancel`, method: 'post' });
}

/** enqueue a new build with the config of the given build */
export function fetchRetryBuild(id: string) {
  return request<{ build: Api.IsoBuilder.Build }>({ url: `/api/builds/${id}/retry`, method: 'post' });
}

/** refresh the artifact, the presigned url is renewed */
export function fetchGetArtifact(id: string) {
  return request<{ artifact: Api.IsoBuilder.Artifact }>({ url: `/api/artifacts/${id}` });
}

/** the url of the artifact download endpoint, it redirects to the presigned url when the storage driver is s3 */
export function getArtifactDownloadUrl(id: string) {
  return buildApiUrl(`/api/artifacts/${id}/download`);
}

/** whether the build is in a terminal state */
export function isBuildFinished(status: Api.IsoBuilder.BuildStatus) {
  return status === 'succeeded' || status === 'failed' || status === 'canceled';
}

export interface BuildLogHandlers {
  /** one log line, `offset` is the byte offset to resume from after this line */
  onLine: (line: Api.IsoBuilder.LogLine) => void;
  /** the stream finished, the build reached a terminal state */
  onEnd: (end: Api.IsoBuilder.LogEnd) => void;
  /** the stream failed, e.g. the network dropped */
  onError: (message: string) => void;
}

/**
 * Join the service base URL with an API path.
 *
 * In production the base URL is `/` (the SPA and the API share an origin), and a naive
 * `${base}${path}` concatenation yields `//api/...`, which a browser resolves as a
 * protocol-relative URL pointing at the host `api` — the log stream and every download
 * link then fail with a connection error while the same code works in dev, where the
 * base URL is the proxy pattern. Strip the trailing slash before joining.
 */
function buildApiUrl(path: string) {
  return `${serviceBaseURL.replace(/\/+$/, '')}${path}`;
}

/**
 * Stream the build log.
 *
 * The endpoint is an SSE stream, but `EventSource` cannot send the authorization header nor read
 * `event: end` reliably, so the stream is parsed by hand from a `fetch` body reader.
 *
 * The returned promise resolves when the stream ends, the caller aborts, or the connection fails.
 */
export async function streamBuildLog(
  buildId: string,
  offset: number,
  signal: AbortSignal,
  handlers: BuildLogHandlers
): Promise<void> {
  const url = buildApiUrl(`/api/builds/${buildId}/log?offset=${offset}`);
  const headers: Record<string, string> = { Accept: 'text/event-stream' };
  const Authorization = getAuthorization();

  if (Authorization) {
    headers.Authorization = Authorization;
  }

  let response: Response;

  try {
    response = await fetch(url, { headers, signal });
  } catch (error) {
    if (isAbortError(error)) return;

    handlers.onError('无法连接日志流，请检查网络后重试。');
    return;
  }

  if (!response.ok) {
    handlers.onError(await readErrorMessage(response));
    return;
  }

  if (!response.body) {
    handlers.onError('当前浏览器不支持流式读取日志。');
    return;
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder('utf-8');
  let buffer = '';
  let ended = false;

  /** parse one `event:` / `data:` block */
  function handleEvent(block: string) {
    let event = 'message';
    const data: string[] = [];

    for (const rawLine of block.split('\n')) {
      const line = rawLine.endsWith('\r') ? rawLine.slice(0, -1) : rawLine;

      if (!line || line.startsWith(':')) continue;

      const separator = line.indexOf(':');
      const field = separator === -1 ? line : line.slice(0, separator);
      let value = separator === -1 ? '' : line.slice(separator + 1);

      if (value.startsWith(' ')) value = value.slice(1);

      if (field === 'event') {
        event = value;
      } else if (field === 'data') {
        data.push(value);
      }
    }

    if (event === 'end') {
      ended = true;
      handlers.onEnd(parseJson<Api.IsoBuilder.LogEnd>(data.join('\n')) || { status: 'succeeded', next_offset: offset });
      return;
    }

    for (const item of data) {
      const line = parseJson<Api.IsoBuilder.LogLine>(item);

      if (line && typeof line.line === 'string') {
        handlers.onLine(line);
      }
    }
  }

  try {
    while (!ended) {
      const { done, value } = await reader.read();

      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      let index = buffer.indexOf('\n\n');

      while (index !== -1) {
        handleEvent(buffer.slice(0, index));
        buffer = buffer.slice(index + 2);

        if (ended) break;

        index = buffer.indexOf('\n\n');
      }
    }
  } catch (error) {
    if (isAbortError(error)) return;

    handlers.onError('日志流已断开，正在尝试重连…');
    return;
  } finally {
    try {
      await reader.cancel();
    } catch {
      // the stream is already closed
    }
  }
}

function parseJson<T>(text: string): T | null {
  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
  }
}

function isAbortError(error: unknown) {
  return Boolean(error && typeof error === 'object' && (error as Error).name === 'AbortError');
}

async function readErrorMessage(response: Response) {
  try {
    const payload = await response.clone().json();

    if (payload?.error?.message) return String(payload.error.message);
  } catch {
    // the body is not a json error payload
  }

  return `日志流请求失败（HTTP ${response.status}）。`;
}
