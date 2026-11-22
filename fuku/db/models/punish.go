package models

// The bounds of a timed automatic punishment (a timed mute or ban of the warn limit, the flood or
// the blacklist), defined once so the modules and the backup import check the same numbers.

// PunishMinSecs is the shortest duration of a timed punishment: 60 seconds, the shortest token
// extraction.ParseDurationToken accepts (1m).
const PunishMinSecs = 60

// PunishMaxSecs is the longest duration of a timed punishment: 366 days (31622400 seconds), the
// longest finite restriction Telegram accepts. A longer one is refused, never made permanent. The
// CHECK constraints of the duration columns allow 0 to this value.
const PunishMaxSecs = 366 * 24 * 60 * 60

// PunishDurationValid reports whether secs is a duration a timed punishment may store:
// PunishMinSecs to PunishMaxSecs inclusive. 0 is not valid (it means no duration, inert).
func PunishDurationValid(secs int) bool {
	return secs >= PunishMinSecs && secs <= PunishMaxSecs
}
