import { describe, expect, it } from "vitest";
import { mergeRepos, parseRepoNames } from "./repoNames";

describe("parseRepoNames", () => {
	it("splits comma-separated input and trims", () => {
		expect(parseRepoNames(" a , b ,c ").valid).toEqual(["a", "b", "c"]);
	});

	it("separates names with illegal characters", () => {
		const got = parseRepoNames("ok-one, bad name, ok.two");
		expect(got.valid).toEqual(["ok-one", "ok.two"]);
		expect(got.rejected).toEqual(["bad name"]);
	});

	it("treats empty and undefined input as no names", () => {
		expect(parseRepoNames("").valid).toEqual([]);
		expect(parseRepoNames(undefined as unknown as string).valid).toEqual([]);
	});
});

describe("mergeRepos", () => {
	// The regression: the user typed a repo and clicked Save without pressing
	// Enter, so the profile was saved with an empty repos list.
	it("keeps a pending name that was never committed with Enter", () => {
		expect(mergeRepos([], "yflow")).toEqual(["yflow"]);
	});

	it("appends the pending name to already committed ones", () => {
		expect(mergeRepos(["a"], "b")).toEqual(["a", "b"]);
	});

	it("does not duplicate a name already committed", () => {
		expect(mergeRepos(["a"], "a")).toEqual(["a"]);
	});

	it("is a no-op when the input is empty", () => {
		expect(mergeRepos(["a"], "  ")).toEqual(["a"]);
	});
});
