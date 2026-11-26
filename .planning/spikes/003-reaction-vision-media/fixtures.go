package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"strings"
)

// MediaFixture is one photo, meme, or sticker thumbnail test case.
type MediaFixture struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Kind         string   `json:"kind"` // "photo", "meme", "sticker"
	MIMEType     string   `json:"mime_type"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	ByteLen      int      `json:"byte_len"`
	Author       string   `json:"author"`
	ChatTitle    string   `json:"chat_title"`
	Language     string   `json:"language"`
	Caption      string   `json:"caption,omitempty"`
	StickerEmoji string   `json:"sticker_emoji,omitempty"`
	RecentLog    []string `json:"recent_log,omitempty"`
	ExpectVision string   `json:"expect_vision"`   // "react" or "none"
	ExpectTextFB string   `json:"expect_text_fb"`  // expected behavior on text-only degradation: "react", "none", or "any"
	Description  string   `json:"description"`
	Data         []byte   `json:"-"`
	DataURL      string   `json:"data_url"` // for UI preview only
}

// 5x7 uppercase ASCII bitmap font for crisp, zero-dependency meme/screenshot rendering.
var font5x7 = map[rune][7]string{
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'B': {"11110", "10001", "10001", "11110", "10001", "10001", "11110"},
	'C': {"01110", "10001", "10000", "10000", "10000", "10001", "01110"},
	'D': {"11100", "10010", "10001", "10001", "10001", "10010", "11100"},
	'E': {"11111", "10000", "10000", "11110", "10000", "10000", "11111"},
	'F': {"11111", "10000", "10000", "11110", "10000", "10000", "10000"},
	'G': {"01110", "10001", "10000", "10111", "10001", "10001", "01110"},
	'H': {"10001", "10001", "10001", "11111", "10001", "10001", "10001"},
	'I': {"01110", "00100", "00100", "00100", "00100", "00100", "01110"},
	'J': {"00111", "00010", "00010", "00010", "10010", "10010", "01100"},
	'K': {"10001", "10010", "10100", "11000", "10100", "10010", "10001"},
	'L': {"10000", "10000", "10000", "10000", "10000", "10000", "11111"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'N': {"10001", "11001", "10101", "10011", "10001", "10001", "10001"},
	'O': {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
	'P': {"11110", "10001", "10001", "11110", "10000", "10000", "10000"},
	'Q': {"01110", "10001", "10001", "10001", "10101", "10010", "01101"},
	'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	'S': {"01111", "10000", "10000", "01110", "00001", "00001", "11110"},
	'T': {"11111", "00100", "00100", "00100", "00100", "00100", "00100"},
	'U': {"10001", "10001", "10001", "10001", "10001", "10001", "01110"},
	'V': {"10001", "10001", "10001", "10001", "10001", "01010", "00100"},
	'W': {"10001", "10001", "10001", "10101", "10101", "11011", "10001"},
	'X': {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
	'Y': {"10001", "10001", "01010", "00100", "00100", "00100", "00100"},
	'Z': {"11111", "00001", "00010", "00100", "01000", "10000", "11111"},
	'0': {"01110", "10011", "10101", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00110", "01000", "10000", "11111"},
	'3': {"11110", "00001", "00001", "01110", "00001", "00001", "11110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "11110", "00001", "00001", "10001", "01110"},
	'6': {"01110", "10000", "11110", "10001", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00001", "01110"},
	':': {"00000", "00100", "00100", "00000", "00100", "00100", "00000"},
	'.': {"00000", "00000", "00000", "00000", "00000", "01100", "01100"},
	'!': {"00100", "00100", "00100", "00100", "00100", "00000", "00100"},
	'-': {"00000", "00000", "00000", "11111", "00000", "00000", "00000"},
	'#': {"01010", "11111", "01010", "01010", "11111", "01010", "00000"},
	'$': {"00100", "01111", "10100", "01110", "00101", "11110", "00100"},
	'/': {"00001", "00010", "00010", "00100", "01000", "01000", "10000"},
	' ': {"00000", "00000", "00000", "00000", "00000", "00000", "00000"},
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func fillTriangle(img *image.RGBA, x0, y0, x1, y1, x2, y2 int, c color.RGBA) {
	minX := min(x0, min(x1, x2))
	maxX := max(x0, max(x1, x2))
	minY := min(y0, min(y1, y2))
	maxY := max(y0, max(y1, y2))
	edge := func(ax, ay, bx, by, px, py int) int { return (px-ax)*(by-ay) - (py-ay)*(bx-ax) }
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			w0 := edge(x0, y0, x1, y1, x, y)
			w1 := edge(x1, y1, x2, y2, x, y)
			w2 := edge(x2, y2, x0, y0, x, y)
			if (w0 >= 0 && w1 >= 0 && w2 >= 0) || (w0 <= 0 && w1 <= 0 && w2 <= 0) {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawText(img *image.RGBA, x, y, scale int, text string, c color.RGBA) {
	cursor := x
	for _, r := range strings.ToUpper(text) {
		glyph, ok := font5x7[r]
		if !ok {
			glyph = font5x7[' ']
		}
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if glyph[row][col] == '1' {
					fillRect(img, cursor+col*scale, y+row*scale, cursor+(col+1)*scale, y+(row+1)*scale, c)
				}
			}
		}
		cursor += 6 * scale
	}
}

func drawTextCentered(img *image.RGBA, y, scale int, text string, c color.RGBA) {
	w := len([]rune(text))*6*scale - scale
	x := (img.Bounds().Dx() - w) / 2
	if x < 4 {
		x = 4
	}
	drawText(img, x, y, scale, text, c)
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func encodeJPEG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}

func makeCatPNG(w, h int, smug bool) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bg := color.RGBA{24, 34, 48, 255}
	if smug {
		bg = color.RGBA{38, 28, 56, 255}
	}
	fillRect(img, 0, 0, w, h, bg)
	cx, cy := w/2, h/2+h/14
	r := min(w, h) / 3
	ginger := color.RGBA{235, 146, 52, 255}
	innerEar := color.RGBA{245, 175, 175, 255}
	// Ears
	fillTriangle(img, cx-r, cy-r/2, cx-r/5, cy-r, cx-r*9/10, cy-r*14/10, ginger)
	fillTriangle(img, cx+r, cy-r/2, cx+r/5, cy-r, cx+r*9/10, cy-r*14/10, ginger)
	fillTriangle(img, cx-r*4/5, cy-r*3/5, cx-r/3, cy-r*9/10, cx-r*3/4, cy-r*11/10, innerEar)
	fillTriangle(img, cx+r*4/5, cy-r*3/5, cx+r/3, cy-r*9/10, cx+r*3/4, cy-r*11/10, innerEar)
	// Head
	fillCircle(img, cx, cy, r, ginger)
	// Eyes
	eyeY := cy - r/6
	eyeR := max(4, r/5)
	fillCircle(img, cx-r/3, eyeY, eyeR, color.RGBA{65, 210, 95, 255})
	fillCircle(img, cx+r/3, eyeY, eyeR, color.RGBA{65, 210, 95, 255})
	fillCircle(img, cx-r/3, eyeY, max(2, eyeR/2), color.RGBA{15, 15, 15, 255})
	fillCircle(img, cx+r/3, eyeY, max(2, eyeR/2), color.RGBA{15, 15, 15, 255})
	if smug {
		// Half-lidded smug eyelids
		fillRect(img, cx-r/3-eyeR, eyeY-eyeR, cx-r/3+eyeR+1, eyeY, ginger)
		fillRect(img, cx+r/3-eyeR, eyeY-eyeR, cx+r/3+eyeR+1, eyeY, ginger)
	}
	// Nose
	fillTriangle(img, cx-eyeR/2, cy+r/10, cx+eyeR/2, cy+r/10, cx, cy+r/4, color.RGBA{240, 120, 145, 255})
	// Whiskers
	wh := color.RGBA{255, 245, 225, 255}
	for _, dy := range []int{-4, 2, 8} {
		fillRect(img, cx-r*6/5, cy+r/5+dy, cx-r/2, cy+r/5+dy+2, wh)
		fillRect(img, cx+r/2, cy+r/5+dy, cx+r*6/5, cy+r/5+dy+2, wh)
	}
	if w >= 240 {
		drawTextCentered(img, h-26, 2, "CAT ON ROUTER", color.RGBA{255, 240, 210, 255})
	}
	return encodePNG(img)
}

func makeFridayMemeJPEG() []byte {
	w, h := 400, 240
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fillRect(img, 0, 0, w, h, color.RGBA{165, 42, 18, 255})
	// Stylized flames at bottom
	orange := color.RGBA{245, 130, 30, 255}
	yellow := color.RGBA{255, 220, 60, 255}
	for i := 0; i < 8; i++ {
		bx := i * 50
		fillTriangle(img, bx, h, bx+50, h, bx+25, h-75, orange)
		fillTriangle(img, bx+10, h, bx+40, h, bx+25, h-45, yellow)
	}
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{15, 15, 15, 255}
	drawTextCentered(img, 24, 3, "DEPLOYING TO PROD", black)
	drawTextCentered(img, 22, 3, "DEPLOYING TO PROD", white)
	drawTextCentered(img, 70, 3, "FRIDAY 17:59", black)
	drawTextCentered(img, 68, 3, "FRIDAY 17:59", yellow)
	drawTextCentered(img, 130, 3, "THIS IS FINE", black)
	drawTextCentered(img, 128, 3, "THIS IS FINE", white)
	return encodeJPEG(img)
}

func makeCrashTerminalPNG() []byte {
	w, h := 400, 220
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fillRect(img, 0, 0, w, h, color.RGBA{20, 12, 14, 255})
	fillRect(img, 0, 0, w, 28, color.RGBA{160, 28, 36, 255})
	drawText(img, 12, 8, 2, "PROD TERMINAL - ALERT", color.RGBA{255, 255, 255, 255})
	drawText(img, 16, 52, 2, "FATAL ERROR 500", color.RGBA{255, 95, 95, 255})
	drawText(img, 16, 86, 2, "PROD DB DROPPED", color.RGBA{255, 180, 100, 255})
	drawText(img, 16, 120, 2, "0 BACKUPS FOUND", color.RGBA{255, 95, 95, 255})
	drawText(img, 16, 162, 2, "GOOD LUCK", color.RGBA{200, 200, 200, 255})
	return encodePNG(img)
}

func makeMundaneInvoicePNG() []byte {
	w, h := 360, 220
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fillRect(img, 0, 0, w, h, color.RGBA{248, 248, 248, 255})
	dark := color.RGBA{40, 40, 45, 255}
	gray := color.RGBA{120, 120, 125, 255}
	fillRect(img, 16, 46, w-16, 48, gray)
	drawText(img, 20, 20, 2, "INVOICE #4821", dark)
	drawText(img, 20, 68, 2, "OFFICE SUPPLIES", gray)
	drawText(img, 20, 102, 2, "SUBTOTAL: $40.00", dark)
	drawText(img, 20, 134, 2, "TOTAL: $42.00", dark)
	drawText(img, 20, 174, 2, "TERMS: NET 30", gray)
	return encodePNG(img)
}

func makeClownStickerPNG() []byte {
	w, h := 128, 128
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fillRect(img, 0, 0, w, h, color.RGBA{25, 28, 36, 255})
	// Red hair puffs
	red := color.RGBA{225, 48, 58, 255}
	fillCircle(img, 26, 44, 18, red)
	fillCircle(img, 102, 44, 18, red)
	// White clown face
	fillCircle(img, 64, 66, 38, color.RGBA{250, 248, 242, 255})
	// Eyes & red nose
	fillCircle(img, 50, 56, 5, color.RGBA{20, 20, 25, 255})
	fillCircle(img, 78, 56, 5, color.RGBA{20, 20, 25, 255})
	fillCircle(img, 64, 68, 9, red)
	// Big smile arc bar
	fillRect(img, 46, 84, 82, 90, red)
	return encodePNG(img)
}

func buildFixtures() []MediaFixture {
	specs := []struct {
		f    MediaFixture
		data []byte
	}{
		{
			f: MediaFixture{
				ID:           "photo_cat_portrait",
				Label:        "Cat photo (no caption)",
				Kind:         "photo",
				MIMEType:     "image/png",
				Width:        320,
				Height:       240,
				Author:       "Mariia",
				ChatTitle:    "General Facility",
				Language:     "English",
				ExpectVision: "react",
				ExpectTextFB: "none",
				Description:  "Zero-caption cat picture — vision providers see the cat (Xenobot-72 hobby); text-only fallback sees only [photo] and should abstain.",
			},
			data: makeCatPNG(320, 240, false),
		},
		{
			f: MediaFixture{
				ID:           "meme_friday_deploy",
				Label:        "Friday deploy meme (JPEG, no caption)",
				Kind:         "meme",
				MIMEType:     "image/jpeg",
				Width:        400,
				Height:       240,
				Author:       "Dmytro",
				ChatTitle:    "Backend Ops",
				Language:     "English",
				ExpectVision: "react",
				ExpectTextFB: "none",
				Description:  "Captionless JPEG meme card ('DEPLOYING TO PROD / FRIDAY 17:59 / THIS IS FINE') — tests multimodal OCR & meme comprehension.",
			},
			data: makeFridayMemeJPEG(),
		},
		{
			f: MediaFixture{
				ID:           "photo_prod_crash",
				Label:        "Crash terminal screenshot + caption",
				Kind:         "photo",
				MIMEType:     "image/png",
				Width:        400,
				Height:       220,
				Author:       "Oleg",
				ChatTitle:    "Backend Ops",
				Language:     "English",
				Caption:      "ops channel right now",
				ExpectVision: "react",
				ExpectTextFB: "any",
				Description:  "Visual crash screenshot ('PROD DB DROPPED / 0 BACKUPS FOUND') paired with minimal caption.",
			},
			data: makeCrashTerminalPNG(),
		},
		{
			f: MediaFixture{
				ID:           "photo_mundane_invoice",
				Label:        "Mundane receipt screenshot",
				Kind:         "photo",
				MIMEType:     "image/png",
				Width:        360,
				Height:       220,
				Author:       "Olena",
				ChatTitle:    "Admin & Finance",
				Language:     "English",
				Caption:      "here is the receipt for office supplies",
				ExpectVision: "none",
				ExpectTextFB: "none",
				Description:  "Boring office invoice — vision providers should see it is routine logistics and still abstain (NONE).",
			},
			data: makeMundaneInvoicePNG(),
		},
		{
			f: MediaFixture{
				ID:           "sticker_smug_cat",
				Label:        "Smug cat sticker thumbnail (128x128)",
				Kind:         "sticker",
				MIMEType:     "image/png",
				Width:        128,
				Height:       128,
				Author:       "Kateryna",
				ChatTitle:    "General Facility",
				Language:     "English",
				StickerEmoji: "😼",
				RecentLog:    []string{"Alex: Who broke the staging bot again?"},
				ExpectVision: "react",
				ExpectTextFB: "any",
				Description:  "Sticker thumbnail + Telegram Sticker.Emoji ('😼') + thread context — tests sticker vision AND clean [sticker 😼] fallback.",
			},
			data: makeCatPNG(128, 128, true),
		},
		{
			f: MediaFixture{
				ID:           "sticker_clown_roast",
				Label:        "Clown sticker reply to bad take",
				Kind:         "sticker",
				MIMEType:     "image/png",
				Width:        128,
				Height:       128,
				Author:       "Bohdan",
				ChatTitle:    "Offtopic",
				Language:     "English",
				StickerEmoji: "🤡",
				RecentLog:    []string{"Chad: Trust me bro, this memecoin is guaranteed 100x by Tuesday."},
				ExpectVision: "react",
				ExpectTextFB: "react",
				Description:  "Clown sticker thumbnail responding to absurd crypto take — both vision and [sticker 🤡] text fallback have strong signal.",
			},
			data: makeClownStickerPNG(),
		},
	}

	out := make([]MediaFixture, 0, len(specs))
	for _, s := range specs {
		f := s.f
		f.Data = s.data
		f.ByteLen = len(s.data)
		f.DataURL = "data:" + f.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(s.data)
		out = append(out, f)
	}
	return out
}
