/**
 * v2 plugin context surface, shared across ywai plugins.
 *
 * These interfaces mirror the subset of the v2 promise-plugin Context
 * (@opencode-ai/plugin) that ywai plugins consume. Structural types are
 * used to stay resilient across v2 beta builds.
 */

export interface V2Location {
	directory: string
	workspaceID?: string
	project?: { id: string; directory: string; canonical: string }
}

export interface V2ToolEditor {
	add(def: {
		name: string
		description?: string
		input?: Record<string, any>
		execute: (input: any, toolCtx?: any) => Promise<any>
	}): void
}

export interface V2Registration {
	dispose(): Promise<void> | void
}

export interface V2SessionContextEvent {
	readonly sessionID: string
	readonly agent: string
	readonly model: Record<string, any>
	system: Array<{ type: "text"; text: string }>
	messages: Array<{
		id?: string
		role: string
		content: Array<Record<string, any>>
	}>
	tools: Record<string, any>
}

export interface V2PluginContext {
	readonly app?: { readonly name: string; readonly version: string }
	readonly location?: V2Location
	session: {
		hook?(
			name: "context" | "prompt",
			cb: (event: any) => Promise<void> | void,
		): Promise<V2Registration>
		get?(input: { sessionID: string }): Promise<any>
		create?(input: { title?: string; parentID?: string }): Promise<any>
		interrupt?(input: { sessionID: string; continue?: boolean }): Promise<any>
		switchModel?(input: {
			sessionID: string
			model: { id: string; providerID: string; variant?: string }
		}): Promise<any>
		switchAgent?(input: { sessionID: string; agent: string }): Promise<any>
		context?(input: { sessionID: string }): Promise<any>
		prompt?(input: Record<string, any>): Promise<any>
		synthetic?(input: Record<string, any>): Promise<any>
		wait?(input: { sessionID: string }): Promise<any>
		delete?(input: { sessionID: string }): Promise<any>
		[key: string]: any
	}
	agent?: {
		list?(): Promise<any>
		transform?(cb: (draft: any) => void): Promise<V2Registration>
		[key: string]: any
	}
	tool?: {
		transform?(cb: (editor: V2ToolEditor) => void): Promise<V2Registration>
		hook?(name: string, cb: (event: any) => Promise<void>): Promise<V2Registration>
		[key: string]: any
	}
	event: {
		subscribe(options?: { signal?: AbortSignal }): AsyncIterable<any>
	}
	provider?: {
		list?(): Promise<any>
		[key: string]: any
	}
	[key: string]: any
}
