---
title: Chat Commands
description: Complete guide to Chat module commands and features
---

# 📦 Chat Commands

**🗨 Conversations**

In a group whose owner turned conversations on, I answer members who talk to me. Briefly. I have no powers in conversation: a chat line from me bans, mutes or changes nothing. In a group whose owner keeps a message log, I also write briefs of what was said.

**Commands for everyone**
- /ask `<text>` — asks me something. /xeno works too.
- /unrollshort `[<N|me|2h>] [pm]` — tells briefly who talked about what, from the message you reply to or over the last `N` messages, since your last message (`me`), or a window such as `2h`; add `pm` for private chat.
- /unrolllong `[<N|me|2h>] [pm]` — gives a detailed brief of the same stretch.
- /referendum `[<topic>]` — opens a short group poll on the topic, replied debate, or recent thread.
- /inspect — gives a deadpan inspection verdict on the replied message, photo, GIF, video, or sticker.
- /forgetme — deletes the notes I keep about you in this group.

**Admin commands**
- /judge `[group|pm|staff]` — gives an AI opinion of who was at fault, from the message you reply to until now, with a score for each person. `group` posts it in the group, `pm` sends it to your private chat, `staff` to the Staff Group; with no word it goes where the owner chose in /settings.

**Message log**
The group's owner turns the message log on in /settings in a private chat with me. While it is on, I keep the newest 1000 messages of the group: texts, captions, a media tag such as [photo], each sender's first name and ID, and edits. Turning the log off deletes them. A message no longer among the last 1000, or one sent before the log was on, cannot be briefed.

**Notes**
I answer a message that mentions me, a reply to one of my conversation replies, and /ask. I start no conversation, and I never talk in a private chat. Only the group's owner can turn conversations on, in /settings in a private chat with me.
What goes to the AI provider the owner chose: for a conversation, the message (with any attached photo, sticker or supported file, or one preview frame of a GIF or video), the message it replies to and the group's recent conversation with me; for a brief or a verdict, the stretch of messages. Always members' first names, the group's title and language, and the date. No IDs and no usernames.
A verdict is an AI opinion and decides nothing: no one is warned, muted or banned because of a score.
If the group's owner turns memories on in /settings, I keep up to 12 short notes per member, only in that group, from what a member tells me about themselves while talking to me, and I use them only when I answer that member. Notes are never used in briefs, verdicts or reactions, and no one else can read them. They go to the AI provider the owner chose, with no IDs and no usernames. /forgetme deletes yours; turning memories or conversations off deletes everyone's.

**Example**
`/ask what is tempe?`
`/unrollshort 2h pm` — sends a 2-hour brief to your private chat.
`/judge pm` — in reply to the first message of the quarrel.

## Available Commands

| Command | Description | Disableable |
|---------|-------------|-------------|
| `/forgetme` | No description available | ❌ |
| `/inspect` | No description available | ❌ |
| `/judge` | No description available | ❌ |
| `/referendum` | No description available | ❌ |
| `/unrolllong` | No description available | ❌ |
| `/unrollshort` | No description available | ❌ |

## Usage Examples

### Basic Usage

```text
/forgetme
/inspect
/judge
```

For detailed command usage, refer to the commands table above.

## Required Permissions

Commands in this module are available to all users unless otherwise specified.

