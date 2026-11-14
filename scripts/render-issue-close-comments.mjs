import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..");
const dataPath = path.join(repoRoot, "scripts", "issue-close-data.json");
const outputPath = path.join(repoRoot, "docs", "issue-close-comments.md");

function issueURL(repo, number) {
  return `https://github.com/${repo}/issues/${number}`;
}

function readJson(jsonPath) {
  return readFile(jsonPath, "utf8").then((raw) => JSON.parse(raw));
}

const shouldWrite = process.argv.includes("--write");
const data = await readJson(dataPath);
const lines = [];

lines.push("# Issue 关单评论模板");
lines.push("");
lines.push(`更新时间：${data.updatedAt}`);
lines.push("");
lines.push("本文档只覆盖当前 **GitHub 仍 open，但当前仓库代码与本地验证已经覆盖** 的 issue。");
lines.push("");
lines.push("适用范围：");
lines.push("");
for (const item of data.closable) {
  lines.push(`- \`#${item.number}\``);
}
lines.push("");
lines.push("统一验证基线：");
lines.push("");
lines.push(`- ${data.updatedAt} 已重新完成本机验证：`);
lines.push(`  - \`${data.verificationBaseline.summaryCommand}\``);
lines.push(`  - \`${data.verificationBaseline.summaryFile ?? "platform-kernel-summary.json"}\`：\`status = ${data.verificationBaseline.summaryStatus}\``);
lines.push("  - 结构化结果：");
for (const file of data.verificationBaseline.resultFiles) {
  lines.push(`    - \`${file}\``);
}
lines.push(`- 前端测试当前为 \`${data.verificationBaseline.frontendTests}\` 通过。`);
lines.push("");
lines.push("建议用法：");
lines.push("");
lines.push("1. 先确认对应功能没有被后续改坏。");
lines.push("2. 复制下面对应 issue 的评论模板。");
lines.push("3. 如仓库维护方式允许，评论后可直接关闭 issue。");
lines.push("");
lines.push("也可以直接使用本地 helper：");
lines.push("");
lines.push("```bash");
lines.push("node scripts/issue-close-helper.mjs list");
lines.push("node scripts/issue-close-helper.mjs comment 24");
lines.push("node scripts/issue-close-helper.mjs plan");
lines.push("node scripts/issue-close-helper.mjs verify-open");
lines.push("node scripts/issue-close-helper.mjs export .tmp/issue-close-export/manual-check");
lines.push("node scripts/render-issue-close-comments.mjs --write");
lines.push("```");

lines.push("");
lines.push("`export` 产物当前会包含：");
lines.push("");
lines.push("- `manifest.json`");
lines.push("- `README.md`");
lines.push("- `plan.json`");
lines.push("- `plan.md`");
for (const item of data.closable) lines.push(`- \`issue-${item.number}.md\``);

lines.push("");
lines.push("如果只是想先确认“当前哪些 issue 会被处理、处理方式是什么”，优先用：");
lines.push("");
lines.push("```bash");
lines.push("node scripts/issue-close-helper.mjs plan");
lines.push(`node scripts/issue-close-helper.mjs plan ${data.closable.slice(0, 2).map((item) => item.number).join(" ")} --comment-only`);
lines.push("```");

lines.push("");
lines.push("`apply` 默认不会执行。只有显式提供 `--execute`，并且环境里存在 `GITHUB_TOKEN` 或 `GH_TOKEN` 时，才会真正对 GitHub 发评论或关闭 issue。例如：");
lines.push("");
lines.push("```bash");
lines.push("node scripts/issue-close-helper.mjs apply 24 --comment-only --execute");
lines.push("node scripts/issue-close-helper.mjs apply all --comment-and-close --execute");
lines.push("```");

lines.push("");
lines.push("建议顺序：");
lines.push("");
lines.push("1. 先跑 `verify-open` 确认当前 open issue 状态没漂移。");
lines.push("2. 再跑 `plan` 看本次准备处理的 issue 列表。");
lines.push("3. 最后才决定是否真的执行 `apply`。");

for (const item of data.closable) {
  lines.push("");
  lines.push(`## \`#${item.number}\` ${item.title}`);
  lines.push("");
  lines.push(`Issue: [#${item.number}](${issueURL(data.upstreamRepo, item.number)})`);
  lines.push("");
  lines.push("```md");
  lines.push(item.comment);
  lines.push("```");
}

lines.push("");
lines.push("## 暂不建议关闭的 open issue");
lines.push("");
for (const item of [...data.holdOpen, ...data.deferred]) {
  lines.push(`- \`#${item.number}\`：${item.reason}。`);
}

const rendered = `${lines.join("\n")}\n`;

if (shouldWrite) {
  await writeFile(outputPath, rendered, "utf8");
  process.stdout.write(`${outputPath}\n`);
} else {
  process.stdout.write(rendered);
}
