import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useWebSocket } from './useWebSocket';

/**
 * Minimal WebSocket stand-in: records instances, lets tests drive the
 * lifecycle handlers, and mirrors the readyState constants the hook reads.
 */
class FakeWebSocket {
	static instances: FakeWebSocket[] = [];
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	readyState = FakeWebSocket.OPEN;
	onopen: (() => void) | null = null;
	onmessage: ((event: { data: string }) => void) | null = null;
	onclose: (() => void) | null = null;
	onerror: ((err: unknown) => void) | null = null;
	sent: string[] = [];

	constructor(public url: string) {
		FakeWebSocket.instances.push(this);
	}

	send(data: string) {
		this.sent.push(data);
	}

	close() {
		this.readyState = FakeWebSocket.CLOSED;
		// Real sockets emit onclose asynchronously; synchronous is fine here
		// because the hook only reads refs inside the handler.
		this.onclose?.();
	}
}

const lastSocket = () =>
	FakeWebSocket.instances[FakeWebSocket.instances.length - 1];

/** Simulate a server-side drop: CLOSED readyState, then the close event. */
const serverDrop = (ws: FakeWebSocket) => {
	ws.readyState = FakeWebSocket.CLOSED;
	ws.onclose?.();
};

describe('useWebSocket', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		FakeWebSocket.instances = [];
		vi.stubGlobal('WebSocket', FakeWebSocket as unknown as typeof WebSocket);
		vi.spyOn(console, 'log').mockImplementation(() => {});
		vi.spyOn(console, 'error').mockImplementation(() => {});
	});

	afterEach(() => {
		vi.restoreAllMocks();
		vi.unstubAllGlobals();
		vi.useRealTimers();
	});

	it('connects on mount using the current origin', () => {
		renderHook(() => useWebSocket('/api/ws', () => {}));
		expect(FakeWebSocket.instances).toHaveLength(1);
		expect(FakeWebSocket.instances[0].url).toBe(
			`ws://${window.location.host}/api/ws`,
		);
	});

	it('reconnects after an unexpected close', () => {
		renderHook(() => useWebSocket('/api/ws', () => {}));
		expect(FakeWebSocket.instances).toHaveLength(1);

		act(() => {
			serverDrop(lastSocket());
		});
		// Not yet: the reconnect waits 3s.
		act(() => {
			vi.advanceTimersByTime(2999);
		});
		expect(FakeWebSocket.instances).toHaveLength(1);

		act(() => {
			vi.advanceTimersByTime(1);
		});
		expect(FakeWebSocket.instances).toHaveLength(2);
	});

	it('does not reconnect after a manual disconnect', () => {
		const { result } = renderHook(() => useWebSocket('/api/ws', () => {}));
		expect(FakeWebSocket.instances).toHaveLength(1);

		act(() => {
			result.current.disconnect();
		});
		expect(lastSocket().readyState).toBe(FakeWebSocket.CLOSED);

		act(() => {
			vi.advanceTimersByTime(10_000);
		});
		expect(FakeWebSocket.instances).toHaveLength(1);
	});

	it('clears the pending reconnect timer on unmount', () => {
		const { unmount } = renderHook(() => useWebSocket('/api/ws', () => {}));

		act(() => {
			// Server-side drop schedules the 3s reconnect.
			serverDrop(lastSocket());
		});
		unmount();

		act(() => {
			vi.advanceTimersByTime(10_000);
		});
		expect(FakeWebSocket.instances).toHaveLength(1);
	});

	it('delivers parsed messages to the callback', () => {
		const onMessage = vi.fn();
		renderHook(() => useWebSocket('/api/ws', onMessage));

		const payload = { type: 'log', payload: 'hello' };
		act(() => {
			lastSocket().onmessage?.({ data: JSON.stringify(payload) });
		});
		expect(onMessage).toHaveBeenCalledWith(payload);
	});

	it('send() serializes to JSON while the socket is open', () => {
		const { result } = renderHook(() => useWebSocket('/api/ws', () => {}));

		act(() => {
			result.current.send({ hello: 1 });
		});
		expect(lastSocket().sent).toEqual([JSON.stringify({ hello: 1 })]);
	});
});
