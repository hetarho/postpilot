---
name: postpilot-naver-pairing
description: Open the signed-in account's Naver Blog editor through the visible dedicated browser without editing or publishing anything.
---

# Postpilot Naver pairing

Use only the visible dedicated Naver browser and keep exactly one page open.

1. Starting from an approved Naver login, home, blog, or blank page, navigate to `https://blog.naver.com/`.
2. Use fresh accessibility snapshots to follow the signed-in account's own blog and its `글쓰기` control. Naver may require opening `내 블로그` first. Prefer visible navigation and controls over constructing an account-specific URL.
3. Stop only when the same page is a real, loaded Naver Blog editor and its current URL is `https://blog.naver.com/PostWriteForm.naver?blogId=...` with a non-empty `blogId` chosen by Naver.
4. Do not type content, upload a file, open publish settings, click `발행`/`등록`/`완료`, or accept a dialog. If login, CAPTCHA, or two-factor verification is required, leave it visible and report that human attention is required.

The final response is informational only. Postpilot independently verifies the live page, account identity, editor structure, and categories; model text is never pairing evidence.
