import React, { useCallback, useEffect, useRef, useState } from 'react';
import { getToken } from '@/services/auth';

interface Props {
  buildId: string;
  /** 构建进入终态后由调用方置 true，用于停止重连 */
  finished?: boolean;
  /** 收到 SSE 的 end 事件时回调，带上最终状态 */
  onStatus?: (status: string) => void;
}

interface LogLineEvent {
  offset: number;
  line: string;
}

const RECONNECT_DELAY = 2000;

/**
 * 构建日志查看器。
 *
 * 后端 `GET /api/builds/{id}/log?offset=N` 返回 SSE：
 *   逐行  data: {"offset":N,"line":"..."}
 *   结束  event: end + data: {"status":"...","next_offset":N}
 *
 * 这里用 fetch + ReadableStream 自行解析，而不是 EventSource：
 * EventSource 无法设置请求头，只能把访问口令塞进 URL，那会把凭据写进
 * 代理与访问日志。fetch 能正常带 Authorization，同时保留断线按 offset 续读。
 */
const BuildLogViewer: React.FC<Props> = ({ buildId, finished, onStatus }) => {
  const [lines, setLines] = useState<string[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);

  const offsetRef = useRef(0);
  const abortRef = useRef<AbortController | null>(null);
  const boxRef = useRef<HTMLPreElement | null>(null);

  const appendLines = useCallback((next: string[]) => {
    if (!next.length) return;
    setLines((prev) => [...prev, ...next]);
  }, []);

  useEffect(() => {
    setLines([]);
    offsetRef.current = 0;

    let stopped = false;
    let timer: number | undefined;

    /** 解析一个 SSE 事件块，返回是否收到终止事件 */
    const handleBlock = (block: string): boolean => {
      let eventName = 'message';
      const dataLines: string[] = [];

      block.split('\n').forEach((raw) => {
        const line = raw.replace(/\r$/, '');
        if (line.startsWith('event:')) {
          eventName = line.slice(6).trim();
        } else if (line.startsWith('data:')) {
          dataLines.push(line.slice(5).trimStart());
        }
      });

      if (!dataLines.length) return false;
      const payload = dataLines.join('\n');

      if (eventName === 'end') {
        try {
          const end = JSON.parse(payload) as { status?: string; next_offset?: number };
          if (typeof end.next_offset === 'number') offsetRef.current = end.next_offset;
          if (end.status) onStatus?.(end.status);
        } catch {
          // 终止事件负载不可解析时忽略，反正流已经结束
        }
        return true;
      }

      try {
        const event = JSON.parse(payload) as LogLineEvent;
        if (typeof event.offset === 'number') offsetRef.current = event.offset;
        if (typeof event.line === 'string') appendLines([event.line]);
      } catch {
        appendLines([payload]);
      }
      return false;
    };

    const run = async () => {
      if (stopped) return;

      const controller = new AbortController();
      abortRef.current = controller;
      setStreaming(true);

      const token = getToken();
      const url = `/api/builds/${buildId}/log?offset=${offsetRef.current}`;

      try {
        const res = await fetch(url, {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
          signal: controller.signal,
        });

        if (!res.ok || !res.body) {
          throw new Error(`日志流返回 HTTP ${res.status}`);
        }

        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let ended = false;

        while (!ended) {
          const { value, done } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });

          // SSE 以空行分隔事件块
          let sep = buffer.indexOf('\n\n');
          while (sep >= 0) {
            const block = buffer.slice(0, sep);
            buffer = buffer.slice(sep + 2);
            if (handleBlock(block)) {
              ended = true;
              break;
            }
            sep = buffer.indexOf('\n\n');
          }
        }

        setStreaming(false);

        if (!ended && !stopped && !finished) {
          // 流意外中断：稍后按当前 offset 续读
          timer = window.setTimeout(run, RECONNECT_DELAY);
        }
      } catch (err) {
        setStreaming(false);
        if (stopped || (err instanceof DOMException && err.name === 'AbortError')) return;
        if (!finished) timer = window.setTimeout(run, RECONNECT_DELAY);
      }
    };

    void run();

    return () => {
      stopped = true;
      if (timer) window.clearTimeout(timer);
      abortRef.current?.abort();
      abortRef.current = null;
    };
  }, [buildId, finished, appendLines, onStatus]);

  useEffect(() => {
    if (!autoScroll || !boxRef.current) return;
    boxRef.current.scrollTop = boxRef.current.scrollHeight;
  }, [lines, autoScroll]);

  return (
    <div>
      <div
        style={{
          marginBottom: 8,
          fontSize: 12,
          color: 'rgba(0,0,0,0.45)',
          display: 'flex',
          justifyContent: 'space-between',
        }}
      >
        <span>
          {streaming ? '实时输出中…' : finished ? '构建已结束' : '等待连接…'}
          {' · '}
          {lines.length} 行
        </span>
        <label style={{ cursor: 'pointer' }}>
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            style={{ marginRight: 4 }}
          />
          自动滚动
        </label>
      </div>
      <pre
        ref={boxRef}
        style={{
          margin: 0,
          padding: 12,
          maxHeight: 560,
          overflow: 'auto',
          background: '#0b1020',
          color: '#d6deeb',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
          fontSize: 12,
          lineHeight: 1.55,
          borderRadius: 6,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
        }}
      >
        {lines.length ? lines.join('\n') : '等待日志输出…'}
      </pre>
    </div>
  );
};

export default BuildLogViewer;
