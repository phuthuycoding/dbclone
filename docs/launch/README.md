# Lịch launch dbclone (bắt đầu 2026-10-07)

Nguồn và rule từng nền tảng: `docs/reference/2026-10-07-launch-platforms.md`.
Text từng nơi: các file cùng thư mục. **Show HN và Reddit anh phải tự viết lại bằng lời
mình** (HN cấm text AI, Reddit phạt post giống marketing); file chỉ là fact sheet.

## Tuần 1 — chuẩn bị

- [x] Repo `homebrew-tap` + `scoop-bucket` tạo xong; goreleaser có `homebrew_casks` + `scoops`.
- [ ] **Anh:** tạo PAT (fine-grained, Contents: Read & write trên `homebrew-tap` và
      `scoop-bucket`) rồi `gh secret set HOMEBREW_TAP_GITHUB_TOKEN -R phuthuycoding/dbclone`.
- [ ] Merge nhánh `phuthuycoding/feat/launch-prep`, tag `v0.3.0` → release tự đẩy cask + manifest.
- [ ] Kiểm tra `brew install phuthuycoding/tap/dbclone` trên máy Mac thật.
- [ ] GIF demo (vhs qua Docker) chèn README + site.
- [ ] Comment thật ở r/golang, r/devops, HN mỗi ngày (không link dbclone).
- [ ] PR awesome-mongodb (`awesome-lists.md`).
- [ ] Email Console.dev + Golang Weekly (`emails.md`).
- [ ] Đăng bài dev.to ở chế độ draft, đọc lại (`dev-to.md`).

## Tuần 2 — launch (giờ VN)

- [ ] Thứ Ba 2026-10-13, 20:00–22:00: Show HN (`show-hn.md`), trực thread ~6 tiếng.
- [ ] Cùng tối: publish dev.to, post X + LinkedIn kèm GIF (`social.md`).
- [ ] Thứ Năm 2026-10-15: r/golang.
- [ ] Thứ Sáu 2026-10-16: comment trong r/selfhosted "New Project Friday".
- [ ] Thứ Bảy 2026-10-17: r/devops. Thứ Hai 19/10: r/mongodb. Thứ Tư 21/10: r/mysql.
- [ ] 2026-10-20..21: gom feedback thành issues, retro.

## Sau đó

- ~2026-11-01: đủ 30 ngày tuổi cho homebrew-core; cần 225 sao (tự submit) hoặc 75 (người khác).
- Có invite Lobsters: post tag `show` + `go` + `databases`, giữ < 25% self-promo.
- 2027-01-02: r/selfhosted post riêng. 2027-03-02: awesome-go (coverage ≥ 90%).
- Driver PostgreSQL → resubmit Show HN.
