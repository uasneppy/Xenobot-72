# Spike Manifest

## Ideas

### contextual-ai-reactions
Add contextual, socially aware AI emoji reactions (and vision-backed reactions on photos/stickers) for Xenobot-72 in Telegram groups without being annoying, spammy, or out of context. Before building Milestone v1.3, validate the three riskiest AI runtime surfaces across Anthropic Claude Haiku, Google Gemini 4 (`thinkingLevel: "low"`), and Featherless AI using Xenobot-72's production leaf provider packages (`xenobot/utils/claude`, `xenobot/utils/gemini`, `xenobot/utils/featherless`).

**Requirements:**
- Offer all 73 standard Telegram reaction emojis (`supportedReactionEmoji` in `xenobot/modules/reactions.go`, normalized by stripping `U+FE0F`), including spicy/edgy ones (`🤡`, `💩`, `🤮`, `🖕`, `🤬`) only when they genuinely fit Xenobot-72's dry persona — strictly intersected with each chat's `ChatFullInfo.AvailableReactions`.
- Include photos and sticker thumbnails on vision-capable providers (Claude / Gemini, and vision-capable Featherless models) reusing `xenobot/modules/chat_media.go`, degrading cleanly to text/caption/sticker-emoji on text-only Featherless models.
- Permission model: Owner enables AI/Conversations per group; once enabled, group admins can toggle individual interactive AI features on/off per chat.
- Default to `NONE` on mundane chatter, ambiguous sarcasm, heated moderation drama, and real tragedies/war news (100% abstention on war/tragedy red lines); keep `IsBig: false`.
- Never let thinking tokens (`gemini-4-hydrogen` `thinkingLevel: "low"`, Qwen3 `<think>` on Featherless) exhaust `MaxTokens` and trigger `ErrNoReply` truncation.
- Reuse `StopChat` (`chatJobs` + `chatContext`) — zero new shutdown drains.

## Spikes

| # | Idea | Name | Type | Validates | Verdict | Tags |
|---|------|------|------|-----------|---------|------|
| 001 | contextual-ai-reactions | reaction-picker-tokens-and-format | standard | Given a compact reaction-picker prompt offering all 73 standard Telegram emojis + NONE, when run across Claude Haiku, Gemini 4 (thinkingLevel: low), and Featherless across MaxTokens (128 / 256 / 512), then responses parse cleanly to one valid standard emoji or NONE with zero ErrNoReply thinking-token truncation and acceptable latency | VALIDATED | ai, reactions, tokens, claude, gemini, featherless |
| 002 | contextual-ai-reactions | reaction-abstention-and-persona | standard | Given multilingual (UK/EN/RU) group snippets covering mundane chatter, jokes/roasts, sarcasm, arguments, and tragedy/war-news red lines, when scored with all 73 emojis available, then red-line and mundane messages return NONE while notable messages earn sharp, in-character reactions (spicy emojis only when deserved) | VALIDATED | ai, reactions, persona, safety, multilingual |
| 003 | contextual-ai-reactions | reaction-vision-media | standard | Given sample photos/memes and sticker thumbnails via the multimodal provider path, when evaluated on vision-capable providers (Claude / Gemini), then reactions match image context within acceptable token cost and degrade cleanly to caption/sticker-emoji on text-only Featherless models | VALIDATED | ai, reactions, vision, multimodal, stickers, featherless |
