import { useEffect, useState } from 'react'
import { configApi } from '../../api/client'

// candidateAgentNames maps a workflow agentRef ("planning/planner-draft") to the
// installed agent names worth trying, in order. Install flattens non-core groups
// into the file name (planning/planner-draft → planning-planner-draft.md) while
// core agents stay bare (core/orchestrator → orchestrator.md), so asking the
// backend for the bare basename alone 404s on grouped agents.
export function candidateAgentNames(ref: string): string[] {
	const parts = ref
		.split('/')
		.map((s) => s.trim())
		.filter(Boolean)
	const bare = parts.pop() ?? ''
	if (!bare) return []
	if (parts.length === 0) return [bare]
	const flat = [...parts, bare].join('-')
	return [...new Set([flat, bare])]
}

// fetchLinkedAgentContent resolves a linked agentRef to its full prompt text.
// Tries each candidate name until one returns content; '' when none resolves.
export async function fetchLinkedAgentContent(ref: string): Promise<string> {
	for (const name of candidateAgentNames(ref)) {
		try {
			const agent = await configApi.getAgent(name)
			if (agent?.content) return agent.content
		} catch {
			// try the next candidate
		}
	}
	return ''
}

// useLinkedAgentContent loads the resolved prompt for a linked node.
// loading is true while the fetch is in flight; content is '' when the node
// is detached or the ref could not be resolved.
export function useLinkedAgentContent(ref: string): { content: string; loading: boolean } {
	const [content, setContent] = useState('')
	const [loading, setLoading] = useState(false)
	useEffect(() => {
		const r = ref.trim()
		if (!r) {
			setContent('')
			setLoading(false)
			return
		}
		let live = true
		setLoading(true)
		fetchLinkedAgentContent(r)
			.then((c) => {
				if (live) setContent(c ?? '')
			})
			.catch(() => {
				if (live) setContent('')
			})
			.finally(() => {
				if (live) setLoading(false)
			})
		return () => {
			live = false
		}
	}, [ref])
	return { content, loading }
}
