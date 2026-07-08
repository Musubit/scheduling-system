# Issue tracker: Local Markdown

Issues 和 PRD 以 Markdown 文件形式存放在 `.scratch/` 目录下。

## 约定

- 每个功能一个目录：`.scratch/<feature-slug>/`
- PRD 文件：`.scratch/<feature-slug>/PRD.md`
- 实现 issue：`.scratch/<feature-slug>/issues/<NN>-<slug>.md`，从 `01` 开始编号
- Triage 状态以 `Status:` 行记录在 issue 文件顶部附近（状态字符串见 `triage-labels.md`）
- 评论和对话历史追加到文件底部 `## Comments` 标题下

## 当 skill 说"发布到 issue tracker"

在 `.scratch/<feature-slug>/` 下创建新文件（如目录不存在则先创建）。

## 当 skill 说"获取相关 ticket"

读取对应路径的文件。用户通常会直接传递文件路径或 issue 编号。
