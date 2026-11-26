# Research Questions

## v1.3 Interactive Presence

- **Backup & Copy Settings parity (Phase 29 / Phase 30)**: Audit `xenobot/modules/copy_settings.go`, `xenobot/modules/backup.go`, and `xenobot/db/backup/` to determine which new v1.3 interactivity settings (admin AI feature toggles, contextual reaction frequency, Emoji Consensus, `👎` retraction, smarter static reaction options, Highlights Archive target/threshold, vocative addressing) should be included in `/export`, `/import`, and Copy Settings — and how sensitive chat-specific targets (e.g., Highlights destination chat/topic ID) and Owner-gated AI enablement should behave on import/copy.
- **Contextual reaction picker live AI behavior (tracked in spike)**: Empirically verify minimum safe `MaxTokens`, end-to-end latency, `NONE`-abstention accuracy (mundane logistics, ambiguous sarcasm, war/tragedy red lines), spicy-emoji restraint across all 73 standard emojis, and photo/sticker vision handling across `claude-haiku-5-5`, `gemini-4-hydrogen` (`thinkingLevel: "low"`), and Featherless before locking production prompts and defaults.
