---
status: complete
phase: 20-conversations
source: [20-01-SUMMARY.md, 20-02-SUMMARY.md, 20-03-SUMMARY.md, 20-04-SUMMARY.md, 20-05-SUMMARY.md, 20-06-SUMMARY.md, 20-07-SUMMARY.md, 20-08-SUMMARY.md]
started: 2026-11-22T18:21:00Z
updated: 2026-11-22T19:02:07.771Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start and Migration
expected: Start the bot from scratch with ENABLE_CHAT=true and an AI provider key set (ANTHROPIC_API_KEY or GEMINI_API_KEY). Migration 20261119120000_add_chat_conversations.sql applies cleanly, the bot boots without errors, and /stats includes the Conversations line.
result: pass

### 2. Creator-Only Private Conversations Page
expected: Open /settings in private chat for a test supergroup where the bot is an admin. Only the group creator sees the Conversations row; a regular admin does not, and group-menu presses cannot write conversation settings.
result: pass

### 3. Enable Confirmation Card and Memory Reset on Off
expected: On the private Conversations page, turning it on shows the data notice card and tapping Yes enables it. Turning it off takes one tap, stops mention replies, and clears conversation history so turning it back on starts fresh.
result: pass

### 4. Mentions, Replies and Thread Continuity
expected: Mentioning the bot in a group message or caption produces one short in-character reply in the user's language as a reply. Replying to that bot message continues the conversation with history, while replying to a ban notice or welcome message gets no answer.
result: pass

### 5. /ask and /xeno Commands
expected: In an enabled group, /ask <question> and /xeno <question> answer; bare /ask shows the usage line; /ask in private shows the private refusal; when conversations are off, /ask shows the off hint; disabling ask disables both /ask and /xeno.
result: pass

### 6. Audience, Deleted Messages and Moderation Refusal
expected: With Audience set to "Admins and approved", a plain member gets no answer while an approved member is answered. A blacklisted word with a mention is deleted and gets no reply. Asking the bot to ban someone makes it refuse without taking action.
result: pass

### 7. Output Filter and Hourly Allowance
expected: Asking the bot for a link or to ping someone replaces links with [link removed] and keeps @names from notifying anyone. After 12 questions in one hour from one account, the 13th triggers the allowance line once.
result: pass

### 8. Persona Check, Gemini Wire Details and Cost Table
expected: Running scripts/chat-spike against 20-PERSONA-CHECK.md on Anthropic (claude-haiku-5-5) and Gemini (gemini-4-hydrogen) confirms character/canon adherence, Gemini thinkingLevel and blocked-prompt parsing, and token counts for the environment docs cost table.
result: pass

## Summary

total: 8
passed: 8
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
