// refract deterministic end-of-turn validation (never invokes an LLM).
// Runs the project validator after each agent run and surfaces failures.
import * as fs from "node:fs";
import * as path from "node:path";
import type { HookAPI } from "@oh-my-pi/pi-coding-agent/extensibility/hooks";

function projectRoot(start: string): string | undefined {
	let dir = path.resolve(start);
	while (true) {
		if (fs.existsSync(path.join(dir, ".refract/bin/agent-check"))) return dir;
		const parent = path.dirname(dir);
		if (parent === dir) return undefined;
		dir = parent;
	}
}

export default function (pi: HookAPI) {
	pi.on("agent_end", async (_event, ctx) => {
		const root = projectRoot(ctx.cwd);
		if (!root) return;
		const { stdout, code } = await pi.exec("python3", ["-I", `${root}/.refract/bin/agent-check`, "--format", "plain"], {
			cwd: root,
			timeout: 120_000,
		});
		if (code !== 0 && ctx.hasUI) {
			ctx.ui.notify(stdout.trim(), "warning");
		}
	});
}
