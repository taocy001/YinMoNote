#!/usr/bin/env bash
# tests/gen-stress-data.sh
#
# Generate N complex Markdown notes that exercise every syntax element
# supported by YinMoNote, for performance and rendering stress tests.
#
# Syntax covered: H1-H6, bold/italic/strikethrough/underline/inline-code/
# highlight/subscript/superscript, ordered/unordered/task lists, blockquotes,
# fenced code blocks (Go/Python/JS/Bash/SQL/YAML/JSON/Rust), KaTeX inline &
# block math, Mermaid diagrams, tables, images, links, callout blocks
# (info/warning/tip/danger), toggle/details blocks, horizontal rules.
#
# Usage:
#   ./tests/gen-stress-data.sh [OPTIONS]
#
# Options:
#   --notes N        Number of notes to generate           (default: 100)
#   --min-size KB    Minimum note size in KB               (default: 20)
#   --max-size KB    Maximum note size in KB               (default: 200)
#   --lang LANG      Content language: en | zh | mixed     (default: mixed)
#   --data-dir PATH  Target notes directory                (default: ~/.yinmonote/notes)
#   -h, --help       Show this help and exit
#
# Requirements: bash, python3
#
# Examples:
#   ./tests/gen-stress-data.sh
#   ./tests/gen-stress-data.sh --notes 500 --lang zh
#   ./tests/gen-stress-data.sh --notes 1000 --min-size 50 --max-size 300 --data-dir /tmp/stress

set -euo pipefail

# ── Argument parsing ──────────────────────────────────────────────────────────
NOTES_COUNT=100
MIN_SIZE_KB=20
MAX_SIZE_KB=200
LANG="mixed"
DATA_DIR="$HOME/.yinmonote/notes"

usage() {
    grep '^#' "$0" | grep -A100 '# Usage:' | grep -B100 '^# Requirements:' \
        | grep -v '^##' | sed 's/^# \{0,1\}//'
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --notes)     NOTES_COUNT="$2";  shift 2 ;;
        --min-size)  MIN_SIZE_KB="$2";  shift 2 ;;
        --max-size)  MAX_SIZE_KB="$2";  shift 2 ;;
        --lang)      LANG="$2";         shift 2 ;;
        --data-dir)  DATA_DIR="$2";     shift 2 ;;
        -h|--help)   usage ;;
        *) echo "Unknown option: $1" >&2; echo "Run with --help for usage." >&2; exit 1 ;;
    esac
done

case "$LANG" in
    en|zh|mixed) ;;
    *) echo "Error: --lang must be one of: en zh mixed" >&2; exit 1 ;;
esac

CONFIG_FILE="$(dirname "$DATA_DIR")/config.json"

echo "--- YinMoNote Stress Data Generator ---"
echo "Notes:    $NOTES_COUNT"
echo "Size:     ${MIN_SIZE_KB}–${MAX_SIZE_KB} KB per note"
echo "Language: $LANG"
echo "Data dir: $DATA_DIR"
echo ""

mkdir -p "$DATA_DIR"

# ── All content generation delegated to Python ───────────────────────────────
python3 - \
    "$DATA_DIR" "$CONFIG_FILE" \
    "$NOTES_COUNT" "$MIN_SIZE_KB" "$MAX_SIZE_KB" \
    "$LANG" \
<<'PYEOF'
import sys, os, json, random, string
from datetime import datetime

data_dir, config_file, count, min_kb, max_kb, lang = (
    sys.argv[1], sys.argv[2], int(sys.argv[3]),
    int(sys.argv[4]), int(sys.argv[5]), sys.argv[6]
)

# ── Prose content pools ───────────────────────────────────────────────────────

EN_PARAGRAPHS = [
    "Knowledge management is the practice of capturing, organizing, and sharing information so that it can be efficiently retrieved and applied. A well-structured note-taking system reduces cognitive load and allows you to focus on higher-order thinking.",
    "Encryption ensures that your private thoughts remain private. End-to-end encryption means that even the service provider cannot read your data — only you hold the key.",
    "Version control is not just for code. Tracking the history of your notes lets you recover deleted ideas, understand how your thinking evolved, and restore previous states with confidence.",
    "A Zettelkasten is a method of knowledge management that emphasizes linking atomic notes together, forming an emergent network of ideas rather than a rigid hierarchy.",
    "Performance optimization begins with measurement. Before making changes, establish a baseline. Profile your application, identify the hottest paths, and focus effort where it matters most.",
    "The best tools are the ones that disappear — you stop thinking about the tool and start thinking about your work. Frictionless capture is the first principle of effective note-taking.",
    "Markdown strikes a balance between human readability and machine processability. Plain text remains accessible decades later, without proprietary lock-in.",
    "Cryptographic hash functions produce a fixed-size digest that uniquely represents an input. Even a single-bit change in the input produces a completely different hash — the avalanche effect.",
]

ZH_PARAGRAPHS = [
    "知识管理是一门将信息系统化捕捉、整理和检索的艺术。好的笔记系统能够降低认知负荷，让思维专注于更高层次的创造与联结，而非反复翻找散落的碎片。",
    "端对端加密意味着只有持有密钥的你才能解读自己的笔记——即便服务器被攻破，攻击者获得的也只是无意义的密文。隐私不是奢侈品，而是数字时代的基本权利。",
    "版本历史不只属于代码仓库。将想法的演变过程记录下来，你就能清楚地看到思维是如何发展的，在任意时刻恢复到某个灵感状态，或追溯一个错误决策的根源。",
    "卡片盒笔记法（Zettelkasten）的核心在于原子化笔记与显式链接的结合。每一张卡片聚焦单一概念，通过有意识的连接，整个系统自然涌现出超越单张卡片的洞见。",
    "性能优化始于度量。在进行任何改动之前，先建立基准线，找到真正的瓶颈，集中精力优化最热路径。过早优化是万恶之源，数据驱动的优化才是正道。",
    "真正好用的工具会让自己变得透明——你不再思考工具本身，而是专注于思想本身。零摩擦的捕捉是高效笔记的第一原则，任何阻碍记录的摩擦都是信息的流失。",
    "Markdown 在人类可读性与机器可处理性之间取得了完美平衡。纯文本格式历久弥新，无需依赖专有软件，几十年后同样可以打开阅读，这是数字笔记最重要的持久性保证。",
    "密码学哈希函数将任意长度的输入映射为固定长度的摘要，且具有雪崩效应：输入哪怕发生一个比特的改变，输出都将面目全非。这一特性是数据完整性验证的基石。",
]

EN_TITLES = [
    "System Design Notes", "Cryptography Fundamentals", "Performance Profiling",
    "Knowledge Graph Theory", "Database Internals", "API Design Patterns",
    "Security Audit Log", "Distributed Systems", "Compiler Construction",
    "Network Protocol Analysis", "Machine Learning Concepts", "Refactoring Techniques",
    "Architecture Decision Record", "Debugging War Stories", "Open Source Contributions",
]

ZH_TITLES = [
    "系统设计笔记", "密码学基础", "性能分析", "知识图谱理论", "数据库内核",
    "API 设计模式", "安全审计日志", "分布式系统", "编译器构造", "网络协议分析",
    "机器学习概念", "重构技巧", "架构决策记录", "调试战记", "开源贡献",
]

def pick_lang():
    if lang == "en":   return "en"
    if lang == "zh":   return "zh"
    return random.choice(["en", "zh"])

def pick_title(idx, note_lang):
    pool = ZH_TITLES if note_lang == "zh" else EN_TITLES
    return f"{pool[idx % len(pool)]} #{idx + 1}"

def pick_para(note_lang):
    pool = ZH_PARAGRAPHS if note_lang == "zh" else EN_PARAGRAPHS
    return random.choice(pool)

# ── Syntax element generators ─────────────────────────────────────────────────

def section_headings(title, note_lang):
    if note_lang == "zh":
        return f"""# {title}
## 一、背景与动机
### 1.1 问题陈述
#### 1.1.1 核心约束
##### 细节说明
###### 补充备注

"""
    return f"""# {title}
## Background and Motivation
### Problem Statement
#### Core Constraints
##### Implementation Detail
###### Supplementary Note

"""

def section_inline_formatting(note_lang):
    if note_lang == "zh":
        return """## 内联格式

这段文字包含 **粗体**、*斜体*、~~删除线~~、<u>下划线</u>、`行内代码`。
同时支持 ==高亮标记==、下标 H~2~O、上标 E=mc^2^，以及 $E = mc^2$ 行内数学公式。

> 引言的力量在于其简洁。当你能用一句话说清楚，就不要用两句话。

"""
    return """## Inline Formatting

This paragraph demonstrates **bold text**, *italic text*, ~~strikethrough~~, <u>underline</u>, `inline code`.
Also: ==highlighted text==, subscript H~2~O, superscript E=mc^2^, and inline math $\\int_0^\\infty e^{-x^2} dx = \\frac{\\sqrt{\\pi}}{2}$.

> The art of communication is the language of leadership. Clarity trumps cleverness every time.

"""

def section_lists(note_lang):
    if note_lang == "zh":
        return """## 列表

**有序列表**

1. 第一步：明确目标
2. 第二步：收集信息
   1. 主要来源
   2. 辅助来源
3. 第三步：整理分析
4. 第四步：输出结论

**无序列表**

- 加密存储
- 版本历史
  - 自动提交
  - 手动快照
- 离线访问
- 全文搜索

**任务清单**

- [x] 实现端对端加密
- [x] 集成版本控制
- [ ] 优化索引性能
- [ ] 添加协同编辑
- [ ] 支持插件系统

"""
    return """## Lists

**Ordered list**

1. Define the problem clearly
2. Gather relevant information
   1. Primary sources
   2. Secondary sources
3. Analyze and synthesize
4. Document conclusions

**Unordered list**

- Encrypted storage
- Version history
  - Automatic commits
  - Manual snapshots
- Offline access
- Full-text search

**Task list**

- [x] Implement end-to-end encryption
- [x] Integrate version control
- [ ] Optimize index performance
- [ ] Add collaborative editing
- [ ] Build plugin system

"""

def section_table(note_lang, idx):
    latency = random.randint(5, 180)
    memory  = random.uniform(10, 120)
    status  = "PASS" if latency < 150 else "WARN"
    if note_lang == "zh":
        return f"""## 数据表格

| 指标 | 数值 | 阈值 | 状态 |
| :--- | ---: | ---: | :---: |
| 延迟 | {latency} ms | 150 ms | {status} |
| 内存 | {memory:.1f} MB | 128 MB | {"正常" if memory < 128 else "警告"} |
| 注记数 | {idx * 7 + 43} | 10 000 | 正常 |
| 索引大小 | {random.randint(1, 50)} MB | 200 MB | 正常 |

"""
    return f"""## Data Table

| Metric | Value | Threshold | Status |
| :--- | ---: | ---: | :---: |
| Latency | {latency} ms | 150 ms | {status} |
| Memory | {memory:.1f} MB | 128 MB | {"OK" if memory < 128 else "WARN"} |
| Note count | {idx * 7 + 43} | 10 000 | OK |
| Index size | {random.randint(1, 50)} MB | 200 MB | OK |

"""

_CODE_GO = r"""```go
package main

import (
    "fmt"
    "net/http"
)

type NoteServer struct {
    dataDir string
    port    string
}

func (s *NoteServer) Run() error {
    fmt.Printf("Listening on %s\n", s.port)
    return http.ListenAndServe(s.port, nil)
}

func main() {
    server := &NoteServer{dataDir: "./data", port: ":8080"}
    if err := server.Run(); err != nil {
        panic(err)
    }
}
```"""

_CODE_PY = r"""```python
import hashlib, json
from pathlib import Path

def compute_integrity(data_dir: str) -> dict:
    results = {}
    for path in Path(data_dir).glob("*.md"):
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        results[path.name] = digest
    return results

if __name__ == "__main__":
    report = compute_integrity("./notes")
    print(json.dumps(report, indent=2))
```"""

_CODE_TS = r"""```typescript
interface Note {
  id: string
  title: string
  content: string
  encryptedAt?: number
}

async function fetchNote(id: string): Promise<Note> {
  const res = await fetch(`/api/notes/${id}`)
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}
```"""

_CODE_SQL = r"""```sql
SELECT
    n.id,
    n.title,
    COUNT(v.id)  AS version_count,
    MAX(v.ts)    AS last_modified
FROM notes n
LEFT JOIN versions v ON v.note_id = n.id
WHERE n.deleted_at IS NULL
GROUP BY n.id, n.title
ORDER BY last_modified DESC
LIMIT 50;
```"""

_CODE_YAML = r"""```yaml
server:
  port: 8080
  data_dir: ~/.yinmonote/notes
  tls:
    enabled: true
    mode: self_signed

auth:
  pbkdf2_iterations: 100000
  session_timeout: 86400

git:
  auto_commit: true
  commit_interval: 30s
```"""

_CODE_RUST = r"""```rust
use std::collections::HashMap;

fn word_frequency(text: &str) -> HashMap<&str, usize> {
    let mut freq = HashMap::new();
    for word in text.split_whitespace() {
        *freq.entry(word).or_insert(0) += 1;
    }
    freq
}
```"""

def section_code_blocks(note_lang):
    if note_lang == "zh":
        heading    = "## 代码示例"
        label_go   = "Go 后端示例"
        label_py   = "Python 分析脚本"
        label_ts   = "TypeScript 前端"
        label_sql  = "SQL 查询"
        label_yaml = "配置文件"
        label_rust = "Rust 示例"
    else:
        heading    = "## Code Examples"
        label_go   = "Go backend example"
        label_py   = "Python analysis script"
        label_ts   = "TypeScript frontend"
        label_sql  = "SQL query"
        label_yaml = "Configuration"
        label_rust = "Rust example"

    parts = [heading, "", label_go, "", _CODE_GO, "",
             label_py, "", _CODE_PY, "",
             label_ts, "", _CODE_TS, "",
             label_sql, "", _CODE_SQL, "",
             label_yaml, "", _CODE_YAML, "",
             label_rust, "", _CODE_RUST, ""]
    return "\n".join(parts) + "\n"

def section_math(note_lang):
    heading = "## 数学公式" if note_lang == "zh" else "## Mathematics"
    desc_inline = "行内公式示例：质能方程 $E = mc^2$，欧拉恒等式 $e^{i\\pi} + 1 = 0$，正态分布 $X \\sim \\mathcal{N}(\\mu, \\sigma^2)$。" \
        if note_lang == "zh" else \
        "Inline math: mass–energy equivalence $E = mc^2$, Euler's identity $e^{i\\pi} + 1 = 0$, and normal distribution $X \\sim \\mathcal{N}(\\mu, \\sigma^2)$."
    desc_block = "块级公式（KaTeX）：" if note_lang == "zh" else "Block math (KaTeX):"

    return f"""{heading}

{desc_inline}

{desc_block}

```math
\\int_{{-\\infty}}^{{\\infty}} e^{{-x^2}} dx = \\sqrt{{\\pi}}
```

```math
\\begin{{pmatrix}} a & b \\\\ c & d \\end{{pmatrix}}
\\begin{{pmatrix}} x \\\\ y \\end{{pmatrix}}
= \\begin{{pmatrix}} ax + by \\\\ cx + dy \\end{{pmatrix}}
```

"""

def section_mermaid(note_lang):
    heading = "## 流程图" if note_lang == "zh" else "## Diagrams"
    label   = "笔记保存流程" if note_lang == "zh" else "Note save flow"
    return f"""{heading}

{label}

```mermaid
flowchart TD
    A([Start]) --> B{{Encrypted?}}
    B -- Yes --> C[PBKDF2 derive key]
    B -- No  --> D[Plaintext write]
    C --> E[AES-GCM encrypt]
    E --> F[Atomic write to disk]
    D --> F
    F --> G[git commit]
    G --> H([Done])
```

```mermaid
sequenceDiagram
    participant C as Client
    participant S as Server
    participant G as Git

    C->>S: POST /api/notes/:id
    S->>S: Validate & sanitize
    S->>G: wt.Add + wt.Commit
    G-->>S: Commit SHA
    S-->>C: 200 OK + SHA
```

"""

def section_callouts(note_lang):
    if note_lang == "zh":
        return """## Callout 块

<div data-callout="info" data-emoji="ℹ️">

**提示**

此功能目前处于实验阶段，API 可能随版本更新发生变化。

</div>

<div data-callout="warning" data-emoji="⚠️">

**注意**

在生产环境中启用调试模式会显著影响性能，并可能泄露敏感信息。

</div>

<div data-callout="tip" data-emoji="💡">

**最佳实践**

为每条笔记设置清晰的标题和标签，能够大幅提升检索效率和关联发现能力。

</div>

<div data-callout="danger" data-emoji="🚨">

**危险操作**

删除笔记目录前，请确保已完整备份。此操作不可逆，版本历史也将一并清除。

</div>

"""
    return """## Callout Blocks

<div data-callout="info" data-emoji="ℹ️">

**Note**

This feature is currently experimental. The API may change in future releases without notice.

</div>

<div data-callout="warning" data-emoji="⚠️">

**Warning**

Enabling debug mode in production significantly degrades performance and may expose sensitive information.

</div>

<div data-callout="tip" data-emoji="💡">

**Best Practice**

Give every note a clear title and relevant tags. Consistent metadata dramatically improves discoverability and serendipitous connections.

</div>

<div data-callout="danger" data-emoji="🚨">

**Danger**

Deleting the notes directory is irreversible. The entire version history will be lost. Always back up before proceeding.

</div>

"""

def section_toggle(note_lang):
    if note_lang == "zh":
        return """## 折叠块

<details open><summary>点击展开：实现细节</summary>

折叠块内可以包含任意内容，包括代码、列表和表格。

- 使用 ProseMirror `details`/`summary` 节点实现
- 支持嵌套内容
- 初始状态由 `open` 属性控制

</details>

<details><summary>加密算法选择说明（默认折叠）</summary>

选择 AES-256-GCM 的理由：

1. AEAD（带关联数据的认证加密），同时提供保密性和完整性
2. 硬件加速（AES-NI 指令集）
3. NIST 和 RFC 8452 推荐标准

</details>

"""
    return """## Toggle Blocks

<details open><summary>Click to expand: Implementation details</summary>

Toggle blocks can contain arbitrary nested content including code, lists, and tables.

- Implemented using ProseMirror `details`/`summary` nodes
- Supports nested block content
- Initial state controlled by the `open` attribute

</details>

<details><summary>Why AES-256-GCM? (collapsed by default)</summary>

Reasons for choosing AES-256-GCM:

1. AEAD — provides both confidentiality and integrity in one pass
2. Hardware acceleration via AES-NI instruction set
3. Recommended by NIST and RFC 8452

</details>

"""

def section_images_links(note_lang):
    heading = "## 图片与链接" if note_lang == "zh" else "## Images and Links"
    img_alt = "架构示意图" if note_lang == "zh" else "Architecture diagram"
    link_text = "查阅官方文档" if note_lang == "zh" else "Official documentation"
    desc = "占位图片（实际使用时替换为上传的资源路径）：" if note_lang == "zh" \
        else "Placeholder image (replace with actual uploaded asset path):"
    return f"""{heading}

{desc}

![{img_alt}](https://via.placeholder.com/800x400)

[{link_text}](https://spec.commonmark.org/)  ·  [GitHub](https://github.com/)

"""

def section_hr(note_lang):
    label = "分隔线示例" if note_lang == "zh" else "Horizontal rules"
    return f"""## {label}

---

"""

def section_filler_para(note_lang, i):
    """Repeated filler block to reach target size."""
    para = pick_para(note_lang)
    if note_lang == "zh":
        return f"""### 延伸阅读 {i}

{para}

- **关键点一**：系统性思维优于碎片化信息堆积
- **关键点二**：定期回顾比单次记录更重要
- **关键点三**：输出是检验理解的最佳方式

| 维度 | 描述 |
| :--- | :--- |
| 频率 | 每日 |
| 深度 | 深度 |
| 范围 | 跨领域 |

"""
    return f"""### Extended Reading {i}

{para}

- **Key point 1**: Systematic thinking beats fragmented information accumulation
- **Key point 2**: Regular review matters more than the initial capture
- **Key point 3**: Writing output is the best test of genuine understanding

| Dimension | Description |
| :--- | :--- |
| Frequency | Daily |
| Depth | Deep work |
| Scope | Cross-domain |

"""

# ── Note generator ────────────────────────────────────────────────────────────

def gen_id():
    d = datetime.now().strftime("%Y%m%d")
    r = "".join(random.choices(string.ascii_lowercase + string.digits, k=16))
    return d + r + ".md"

def gen_note(idx, note_lang, target_bytes):
    parts = [
        section_headings(pick_title(idx, note_lang), note_lang),
        section_inline_formatting(note_lang),
        section_lists(note_lang),
        section_table(note_lang, idx),
        section_code_blocks(note_lang),
        section_math(note_lang),
        section_mermaid(note_lang),
        section_callouts(note_lang),
        section_toggle(note_lang),
        section_images_links(note_lang),
        section_hr(note_lang),
    ]
    content = "".join(parts)
    filler_idx = 1
    while len(content.encode()) < target_bytes:
        content += section_filler_para(note_lang, filler_idx)
        filler_idx += 1
    return content

# ── Update config quotas ──────────────────────────────────────────────────────
if os.path.exists(config_file):
    with open(config_file) as f:
        cfg = json.load(f)
    cfg["maxTotalNotes"]    = max(cfg.get("maxTotalNotes", 0),    count + 100)
    cfg["maxItemsPerLevel"] = max(cfg.get("maxItemsPerLevel", 0), count + 100)
    cfg["maxNoteSize"]      = 1024 * 1024
    with open(config_file, "w") as f:
        json.dump(cfg, f, indent=2)
    print("[Config] Updated quotas in config.json")
else:
    print(f"[Config] {config_file} not found — skipping quota update")

# ── Generate notes ────────────────────────────────────────────────────────────
ids      = []
parents  = {}
child_order = {}
early_ids = []

print(f"[Generation] Creating {count} notes...")

for i in range(count):
    note_lang  = pick_lang()
    target_min = min_kb * 1024
    target_max = max_kb * 1024
    target     = random.randint(target_min, target_max)

    note_id  = gen_id()
    content  = gen_note(i, note_lang, target)

    with open(os.path.join(data_dir, note_id), "w", encoding="utf-8") as f:
        f.write(content)

    ids.append(note_id)
    if len(early_ids) < 50:
        early_ids.append(note_id)

    # Sparse hierarchy: every ~15th note after the first 50 gets a random parent
    if i > 50 and i % 15 == 0 and early_ids:
        parent = random.choice(early_ids)
        parents[note_id] = parent
        child_order.setdefault(parent, []).append(note_id)

    if (i + 1) % 50 == 0 or (i + 1) == count:
        actual_kb = len(content.encode()) / 1024
        print(f"  - {i + 1:4d} / {count}  (last note: {actual_kb:.0f} KB)")

# ── Write _structure.json ─────────────────────────────────────────────────────
top_level = [nid for nid in ids if nid not in parents]
structure = {
    "order":      top_level,
    "parents":    parents,
    "childOrder": child_order,
    "dark":       False,
}
with open(os.path.join(data_dir, "_structure.json"), "w", encoding="utf-8") as f:
    json.dump(structure, f, indent=2, ensure_ascii=False)

# ── Summary ───────────────────────────────────────────────────────────────────
import glob
md_files   = glob.glob(os.path.join(data_dir, "*.md"))
total_mb   = sum(os.path.getsize(p) for p in md_files) / 1024 / 1024

print()
print("[Summary] Done.")
print(f"  Notes created : {count}")
print(f"  Total MD size : {total_mb:.1f} MB")
print(f"  Data directory: {data_dir}")
PYEOF
