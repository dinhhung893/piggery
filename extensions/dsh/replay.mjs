// Test helper: the committed dsh captures (testdata/fixtures/dsh/0.2.0-rc.1/events-<scenario>.jsonl,
// the sdk profile's session.event notifications) as a list of session events.
import { readFileSync } from "node:fs";

export function events(scenario) {
	const file = new URL(`../../testdata/fixtures/dsh/0.2.0-rc.1/events-${scenario}.jsonl`, import.meta.url);
	return readFileSync(file, "utf8")
		.split("\n")
		.filter(Boolean)
		.map((l) => JSON.parse(l))
		.filter((m) => m.method === "session.event")
		.map((m) => m.params.event);
}
