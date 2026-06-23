# Thanos — Framework Phát Triển Phần Mềm Đa AI Agent cho Go, Codex và Claude Code

[English](README.md) · [Tài liệu kỹ thuật](Technical.md)

Thanos là framework phát triển phần mềm đa AI agent mã nguồn mở, được viết bằng
Go. Framework điều phối các AI coding agent chuyên biệt qua một quy trình kỹ
thuật phần mềm có kiểm soát:

```text
Thiết kế → Duyệt thiết kế → Lập trình → Review → Kiểm thử → Deep Review → Nghiệm thu
                                  ↑           │          │              │
                                  └───────────┴──────────┴──────────────┘
                                                   Sửa lỗi
```

Thanos phù hợp khi một AI agent duy nhất không đủ đáng tin cậy. Thay vì để cùng
một model tự thiết kế, viết code, review và phê duyệt kết quả của chính nó,
Thanos tách công việc thành các vai trò độc lập với input, output và quality gate
rõ ràng.

Thanos hỗ trợ Codex, Claude Code, Cursor, Gemini CLI và runner tùy chỉnh. Công cụ
cũng có thể cài Agent Skills từ GitHub, đồng bộ skill giữa nhiều AI coding agent
và quản lý Claude Code plugin marketplace.

## Tại sao nên dùng Thanos?

Quy trình AI coding một agent thường nhanh nhưng có các rủi ro:

- Code kế thừa sai sót từ thiết kế ban đầu.
- Agent tự review chính giả định của mình.
- Test có thể xác nhận implementation thay vì yêu cầu thực tế.
- Phiên làm việc bị gián đoạn làm mất context và tiến độ.
- Skill bị sao chép và lệch phiên bản giữa Codex, Claude Code, Cursor và Gemini.

Thanos đưa các kiểm soát quan trọng vào Go CLI. AI model phụ trách suy luận và
sinh code; Thanos phụ trách state machine, dependency, giới hạn vòng sửa lỗi,
kiểm tra artifact, khôi phục tiến trình và bước phê duyệt của con người.

## Tính năng chính

- **Phát triển phần mềm đa AI agent:** các vai trò Designer, Coder, Reviewer,
  Tester, Deep Reviewer và Acceptor hoạt động độc lập.
- **Quality gate deterministic:** chuyển phase và report bắt buộc được kiểm tra
  bằng Go, không phụ thuộc vào việc AI có tuân thủ prompt hay không.
- **Không phụ thuộc nhà cung cấp AI:** chạy với Codex, Claude Code, Cursor,
  Gemini CLI hoặc executable tùy chỉnh.
- **Khôi phục sau gián đoạn:** state, event, prompt và report được lưu trong
  `.thanos/`.
- **Codebase graph cục bộ:** lập chỉ mục file, symbol, lời gọi hàm, import, test,
  hub symbol và convention của repository.
- **Review đối kháng:** review thông thường và deep review kiểm tra các nhóm lỗi
  khác nhau.
- **Human-in-the-loop:** feature dừng ở `pending-review` cho đến khi người dùng
  chạy `thanos done`.
- **Cài Agent Skills từ GitHub:** tìm kiếm và cài skill qua hệ sinh thái
  `npx skills`.
- **Đồng bộ skill đa runner:** dùng một nguồn skill chuẩn và liên kết vào thư
  mục riêng của từng AI agent.
- **Claude Code plugins:** thêm marketplace và cài plugin theo scope project.
- **Không cần AI SDK:** runner giao tiếp qua prompt, file và process exit code.

## Cài đặt

Yêu cầu Go 1.20 trở lên.

```bash
go install github.com/tinhtran/thanos/cmd/thanos@latest
```

Build từ source:

```bash
git clone https://github.com/tinhtran/thanos.git
cd thanos
make check
./bin/thanos help
```

## Bắt đầu nhanh

Khởi tạo Thanos trong project hiện có:

```bash
cd your-project
thanos init --runner codex --runner-command codex
```

Với project đã có source code, Thanos tự động tạo:

```text
.thanos/codebase/graph.json
.thanos/codebase/summary.md
```

Tạo feature:

```bash
thanos new "OAuth2 authentication" \
  --description "Thêm Google OAuth2 login và protected sessions." \
  --acceptance "Login thành công;Từ chối state không hợp lệ;Test pass"
```

Chạy toàn bộ quy trình AI development:

```bash
thanos run F001
thanos status
```

Sau khi kiểm tra code và report:

```bash
thanos done F001
```

Nếu tiến trình bị ngắt, `thanos run` tiếp tục từ
`.thanos/<feature-id>/state.json`.

## Codebase Graph cục bộ cho AI Agent

Codebase là cấu trúc, không chỉ là văn bản. Thanos lưu source file, ngôn ngữ,
symbol, function call, import, quan hệ test, hub symbol và convention của
repository.

Mỗi role được yêu cầu đọc `.thanos/codebase/summary.md` trước khi khám phá source
code. Graph đầy đủ cho máy đọc nằm tại `.thanos/codebase/graph.json`.

Quét lại thủ công sau thay đổi lớn:

```bash
thanos scan
```

Graph cũng được tự động làm mới sau khi feature vượt qua acceptance. Toàn bộ dữ
liệu ở local, không cần SaaS, API key hoặc upload source code.

## Các vai trò AI Agent

| Vai trò | Trách nhiệm | Output chính |
|---|---|---|
| Designer | Chuyển yêu cầu thành phạm vi triển khai cụ thể | Task brief, acceptance criteria, test strategy |
| Design Reviewer | Phát hiện thiếu sót kiến trúc trước khi code | `design-review-report.md` |
| Coder | Triển khai đúng task brief đã duyệt | Source code và `coder-report.md` |
| Reviewer | Kiểm tra correctness, security, scope và project rules | `review-report.md` |
| Tester | Xác minh từng acceptance criterion bằng evidence | `test-report.md` |
| Deep Reviewer | Review đối kháng, cross-file và kiến trúc | `deep-review-report.md` |
| Acceptor | Tổng hợp mức độ sẵn sàng và issue còn mở | `final-report.md` |

Thanos còn có prompt chuyên biệt cho Mini-Coder, Re-Verifier, Synthesizer và
Evolution Gate.

## Cài Agent Skills từ GitHub

Tìm skill:

```bash
thanos skill find golang
thanos skill find security
```

Cài skill từ GitHub:

```bash
thanos skill add abc/skill
```

Cài một skill cụ thể và chỉ áp dụng cho một số vai trò:

```bash
thanos skill add vercel-labs/agent-skills \
  --skill web-design-guidelines \
  --roles designer,coder,reviewer
```

Thanos sử dụng Skills CLI mã nguồn mở:

```bash
npx skills add owner/repo --agent universal --yes --copy
```

Các file `SKILL.md` được phát hiện sẽ được ghi vào `.thanos/settings.json` và
đưa vào prompt của các role phù hợp.

## Đồng bộ Skill giữa Codex, Claude Code, Cursor và Gemini

Thêm AI runner:

```bash
thanos runner add claude --command claude
thanos runner add codex --command codex
```

Thanos liên kết skill từ thư mục chuẩn của project vào thư mục native của từng
runner:

| Runner | Thư mục skill |
|---|---|
| Claude Code | `.claude/skills/` |
| Codex | `.agents/skills/` |
| Cursor | `.agents/skills/` |
| Gemini CLI | `.agents/skills/` |

Thêm runner tùy chỉnh:

```bash
thanos runner add custom-agent \
  --command custom-agent \
  --agent custom-agent \
  --skills-dir .custom-agent/skills
```

Relative symlink giúp duy trì một nguồn dữ liệu duy nhất. Thanos không ghi đè
thư mục skill thật đã tồn tại.

## Quản lý Claude Code Plugin

Thêm Claude Code plugin marketplace:

```bash
thanos plugin marketplace add claude anthropics/claude-code
```

Cài plugin cho project:

```bash
thanos plugin install claude \
  commit-commands@claude-code-plugins \
  --scope project
```

Thanos gọi Claude Code plugin CLI chính thức và ghi lại thao tác thành công trong
`.thanos/settings.json`.

## File Protocol cho AI Agent

```text
.thanos/
├── settings.json
├── features/
│   └── F001-oauth2-authentication.yaml
└── F001-oauth2-authentication/
    ├── state.json
    ├── events.jsonl
    ├── task-brief.md
    ├── acceptance-criteria.md
    ├── test-strategy.yaml
    ├── design-review-report.md
    ├── final-report.md
    ├── retro-learnings.json
    └── rounds/
        └── round-1/
            ├── coder-report.md
            ├── review-report.md
            ├── test-report.md
            └── deep-review-report.md
```

Filesystem là nguồn dữ liệu chính. Các agent không cần hidden shared context và
process bị lỗi có thể chạy lại từ phase đã được xác nhận gần nhất.

## Danh sách lệnh

| Lệnh | Mô tả |
|---|---|
| `thanos init` | Khởi tạo workspace, không cần network |
| `thanos new` | Tạo feature specification |
| `thanos run` | Chạy hoặc tiếp tục multi-agent workflow |
| `thanos status` | Xem phase và round hiện tại |
| `thanos prompt` | Render prompt mà không chạy runner |
| `thanos transition` | Chuyển phase thủ công có kiểm tra |
| `thanos done` | Phê duyệt feature đang chờ review |
| `thanos doctor` | Kiểm tra executable của runner |
| `thanos scan` | Tạo hoặc làm mới codebase graph cục bộ |
| `thanos skill find` | Tìm Agent Skills |
| `thanos skill add` | Cài và đăng ký skill từ Git hoặc local source |
| `thanos runner add` | Thêm runner và đồng bộ skill hiện có |
| `thanos plugin marketplace add` | Thêm plugin marketplace |
| `thanos plugin install` | Cài và ghi nhận plugin |

Xem cấu hình chi tiết, runner contract và safety behavior trong
[Tài liệu kỹ thuật](Technical.md).

## Trường hợp sử dụng

Thanos phù hợp với:

- Authentication, authorization và feature nhạy cảm về bảo mật.
- Payment, billing, migration và công việc liên quan data integrity.
- Pull request do AI tạo nhưng cần review độc lập.
- Team cần audit trail và evidence cho code do AI sinh.
- Feature dài, cần tiếp tục sau khi AI session bị gián đoạn.
- Developer sử dụng nhiều AI coding agent nhưng muốn chia sẻ cùng một bộ skill.

Với bản sửa một dòng hoặc prototype nhanh, chạy trực tiếp một agent có thể phù
hợp hơn.

## An toàn

AI runner và plugin chạy với quyền của người dùng trên hệ điều hành. Thanos kiểm
soát workflow nhưng không phải OS sandbox. Hãy review skill và plugin bên thứ ba,
sử dụng branch hoặc worktree riêng, đồng thời kiểm tra code trước khi chạy
`thanos done`.

## Phát triển

```bash
make build
make test
make lint
make check
```

## Nguồn cảm hứng và tiêu chuẩn
- Tích hợp Agent Skills qua [vercel-labs/skills](https://github.com/vercel-labs/skills)
- Claude Code plugin qua [plugin system chính thức](https://code.claude.com/docs/en/discover-plugins)

## Giấy phép

Thanos được phát hành theo [MIT License](LICENSE).
