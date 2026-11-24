//go:build testtools

package modules

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/uasneppy/Xenobot-72/xenobot/db/deletion"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// settingsDelGoldenDigests are the SHA-256 digests of the text and the keyboard JSON of every Phase 16
// deletion screen, recorded once from the code as it was before Phase 17 touched the renderer
// (plan 17-09, M31). The optional row fields of Phase 17 are empty for every Phase 16 row, so none of
// these screens may change; the digests are never re-recorded to make a change pass.
var settingsDelGoldenDigests = map[string]string{
	"list b none can":      "e543e80372e9412621a6e89b1bb50ef4b6acbc595ab0adba51acac1b5145da29",
	"list b none cannot":   "78bf439ea020cdb5cb7df4b6dfd327218639ebb32086bbcd81596d907af93006",
	"list b stored can":    "edf8f6304827869e9286bb7430d43d77e9495b202be4ed7daaca19bdee7d6daf",
	"list b stored cannot": "7a1748a120695f39ffbf7038498f6550b165eb476218e01136aeb329ed4ea0ea",
	"list s none can":      "e60f83a366ea56a350729bf5e98a3b1a0067458a83d620aaf2446447aac4aafb",
	"list s none cannot":   "80d145e860ce73e8d0d24f9fb77e2eb64afec0e19cde46136c51a4b0123a65ce",
	"list s stored can":    "c4e4c6b4eadb97263735b97b719d675c34cef8b54fc9143ddc4d974ad0d5658b",
	"list s stored cannot": "a5cfe899a0d76ce76abf08d0f554d6e95f0e3c786c3e33318307968c2a92e81b",
	"row ba none can":      "05dda302a037219417f6465353a006ad103e9fd56776d6fb2bffd9fea562b453",
	"row ba none cannot":   "b65d1129e8113dd9a5620d88bf2f502b8ce0e86b72918b6eac75e34ad26b5437",
	"row ba stored can":    "5a7ff927b4a87e2a8078385821a45fd5205ce084c649131c5c78cb189b2811fd",
	"row ba stored cannot": "b951fedcb55ee0520fa5ea31b1bf25c73c3c45443419f14b266abad64f7c6aa1",
	"row bg none can":      "6e750d13e693d97cb19edf1e2fa5da40d6e4f840cf7105c6fc10e9a1de1b9b6f",
	"row bg none cannot":   "8c6d7ebf9a1e5422db1756d0828936bb1241f666b35557bc14ad8345e730fefa",
	"row bg stored can":    "3effc79e92137847b953d8bd920b5d3094ebc0ecaf98be33e1edb2e6201f44d5",
	"row bg stored cannot": "0cedf45df47cfac4ce8cc659bfef52e224245dd3d42faeadded478b135e32c46",
	"row bm none can":      "8add9f3590ba8bd54b18653b5345d6530a93ca8ee8fa3d4328fb89c314fcb023",
	"row bm none cannot":   "929bd1f34b92b6e37d42492db2a67f8a2da3e49cb139cb186a2c824aed0c3cb2",
	"row bm stored can":    "7b6163f8089c1378c7e143f9ab5dc95a1d0e68aab5f0d476daf80303834ebc08",
	"row bm stored cannot": "88727ee813c2a70db09a59953942264b24b8bfef8f4a81b466f1d05aa695d43a",
	"row br none can":      "27ded2006589f199b6893d182d18235846c3818a31ffcf6c440cd6fafdebd5c9",
	"row br none cannot":   "bd36fdeb657c9705abf26e2c84fe7f59ffb598c6d8c8eb67a0203cd918d114f9",
	"row br stored can":    "8a99a2e70c519364441cb716494e4559c6519d138b91b6aa2733dfe7ede20845",
	"row br stored cannot": "dcdf64903324e65dc1ecd227633770d10cc0440cfd8b06aa2774cee3bde85033",
	"row bw none can":      "0be9a286c2c0345e97064f82422a0acb10b65d302ec78edcde72176712da8f3d",
	"row bw none cannot":   "770043c766ddbbb80a635ad826e1657f549076fa66e7a00e6be80d09948e57bf",
	"row bw stored can":    "d46f2418ca8fa15c3c8ffbd34e8afb95d29a4bc8c32630a3a08ecbcb85356a8d",
	"row bw stored cannot": "2b452949eeffd88341e12b01ec0bc041f56eb07f3788e57e4e487cfd567a2abf",
	"row sa none can":      "de3b510d5e20188ab3eee931274a8584af637fd7e68c0c1531b344bc139578a3",
	"row sa none cannot":   "20753ad278f5e958e7d271eb521db10e1918c50faf686fde464fb37fdbdc5c92",
	"row sa stored can":    "f14691e58d37ee412ed9715c431ffd4072ef4ae3caf9d4bc438c8374c713ec9e",
	"row sa stored cannot": "c486fb0594cf220f8947dbf5679af323d45521e2b41a6bda09a076855e422fce",
	"row sb none can":      "f4df47a21eed9161d48395e7ed783b2db14217448b32820b0ebe8e9ed2588d3e",
	"row sb none cannot":   "afad085ec2ab79c9aad00a528d080def0d21d0cca87be8877172fd5090ef36ca",
	"row sb stored can":    "a02ed0d73aff59df3cd50d5390dfcb5091f65d43981c6206e79374d490744a52",
	"row sb stored cannot": "1eb988ebc600b49109d28d50a87362792834c92c56427ad6977080b0ab8c06cc",
	"row sf none can":      "06f06f409e2743152ace3afcb9bc279f529e29adc382783b7d2221bfcb020a81",
	"row sf none cannot":   "4e54d48e6263b8dc9aa3100989d72211f155457b038098aaf51fabf1cf96ba77",
	"row sf stored can":    "f6e8dfff5fb53528adb47b5b703eccf02d143f78ee4bc03ecc1517a4090e8b6e",
	"row sf stored cannot": "0b9e6d3472c88f9953a8801af87fd39ae95a291a32155e76cc858e77c9c209d6",
	"row sg none can":      "37205327fb11a1782676197f7b9038d2aeb55acea1ce8aee8a92a961297e580c",
	"row sg none cannot":   "b7fb57edc9396849c8c5fd26999c88754d7ebe94cea74aabd5631f4481a3ac75",
	"row sg stored can":    "744917391d9b005040a8d6d043a60dcc1f78257f93a23702fa883c0ae8a39444",
	"row sg stored cannot": "83adc6a5fe52aa7e918f4126aaf3520d675ff667f24d81b563fbd3a2e2270c9a",
	"row sh none can":      "2af71d30ed5c9b7c2a9895e50f238c532fa3d3c8ce658fa0940fd0e0b1c5f5af",
	"row sh none cannot":   "1bedfb4fabe7c2cec5d91531b5073ba3d8e5f25f0fc8badb01be068aceeebbb5",
	"row sh stored can":    "b75c5b765bd21ab609895429947eda2616e2e06f2f4cb6c61770c2c339e86712",
	"row sh stored cannot": "7faad4f96ef9f72b9a126133492de95948775869056310af5cb9f84b3848f86d",
	"row sj none can":      "58533bcde0b17804a7bec7daaf83cad79f9166470f73f1b43b6d0724b979f454",
	"row sj none cannot":   "995f5083c7b59120a147ec14d3b388c9b2d952e57db1b08d443cbaed118b0f93",
	"row sj stored can":    "95c20b8c404bca673d6b51cb656b065f048473ee4b17ee252f5de1fafa9295f9",
	"row sj stored cannot": "db21e976635a9bde95dc667d1f98d8cdfb674c2487f151151c2e665a3696de1e",
	"row sk none can":      "52592b3b68c17c89521885946de46e78b6c96dde74cb5d4abf913f5ea0005ffa",
	"row sk none cannot":   "e5866bd23e2e305f062ee78279c02b86c29c5e86390c618bb859291bddf62c34",
	"row sk stored can":    "bf33e9446bb91c4d163ffeddd05589dddb46f4c9a88833c076564771feeeca0f",
	"row sk stored cannot": "187ea9e84bcefa178b39802e8350ccb1f52b29911505e758dde7de94b117d63b",
	"row sl none can":      "ad1c3022277568d6f0ad262bb4a5e5e901a5f2d210a291418f8cd06ef74afc0b",
	"row sl none cannot":   "21310f875ddc1ea0b877d4c657a35ed6c6c19ab198ffd543da2c597c03afc001",
	"row sl stored can":    "71317b5ea6c956a6d07bca0cfc4823b28c6fb4410fcb7f8dfd4ba6ab67b227e0",
	"row sl stored cannot": "f010090768a2a29b22f62ccc061161e43dd9c54424335ba9f03471036b2c00fa",
	"row sn none can":      "7bd98e000b6803dcde59a19922fe76ee8e874aa42d53f20223610715e6f8afda",
	"row sn none cannot":   "1587fd8abb24ce15e0bc0bfcd590f289cb3bd97ce7311b14e4cb7626cc28f5d8",
	"row sn stored can":    "9dbd7634635bc96ac868347b6aa1efbfd79c6db6666bcbba238eaf7273fc9f5d",
	"row sn stored cannot": "78476e30efacd0c156dc4a0fe55f3948df8c6e0921f85d620a77d60bfd24442c",
	"row so none can":      "8b2a2cffab3da0a4f427fa8af561835202d7c3c6ca60daad51df300f22856ac2",
	"row so none cannot":   "c854b0fa0b700b6d9ba624c34e571ad05dc3c01006b0711e5c0c39619436be34",
	"row so stored can":    "7cee53b1069eaf522af611b4fda0bcfbf77145c882f0e97c433f808ff1c5f879",
	"row so stored cannot": "2c5366b6438d1c04d39e80356944571b23733a308e8848435c321aba94428be0",
	"row sp none can":      "c3b53d7b4e9398e74cd4284f130af471cf949fca1213fb9891a4df01a5c9eedb",
	"row sp none cannot":   "61955aea57557ef2229d9bfa44f80574979b6992b1c1f0f84a7989b4778ca91d",
	"row sp stored can":    "94b190557d9f92131ce1ddb33e9043ba08e0e572ef36a5ce4769907ea02d9b2e",
	"row sp stored cannot": "7c7f7b245df7ce1d51c9feb428ffbc60db382cb73d7b957676252070cce15a30",
	"row sv none can":      "988e438dad1941b4c21bccb965170a9017b5871b19f598469d25b622f04b4d79",
	"row sv none cannot":   "4922914db970531de2c5f775a6266caf8fd7326f40b1e3bf05644c8c799e6a81",
	"row sv stored can":    "0a92487060dedd401f019ab203e9f5c70796df3eed2fa4ebff40ac2c2839306c",
	"row sv stored cannot": "2b32e1c66aafff53d2b83c6b4e335c3d03a6538a1582adc223a7fe2f9c2357c6",
}

// settingsDelGoldenData is the data of a state of the group: no rule at all, or every Phase 16 row set
// to its first preset (and the shared /cleanservice flag on, so joined and left read it too).
func settingsDelGoldenData(stored bool, rows []deletionRow) settingsDelData {
	data := settingsDelData{Rules: &deletion.Rules{ChatID: 1, Delays: map[string]int{}}}
	if !stored {
		return data
	}
	for _, row := range rows {
		data.Rules.Delays[row.Kind] = row.Presets[0]
	}
	data.Flag = true
	return data
}

// settingsDelGoldenDigest is the digest of one screen: its text and its keyboard as JSON.
func settingsDelGoldenDigest(t *testing.T, screen settingsScreen) string {
	t.Helper()
	keyboard, err := json.Marshal(screen.Keyboard)
	if err != nil {
		t.Fatalf("marshal keyboard: %v", err)
	}
	sum := sha256.Sum256([]byte(screen.Text + "\n--\n" + string(keyboard)))
	return hex.EncodeToString(sum[:])
}

// settingsDelGoldenScreens renders the two Phase 16 lists and the 17 Phase 16 row screens, each with no
// rule and with its first preset stored, each with no right line and with the cannot-delete line.
func settingsDelGoldenScreens(t *testing.T) map[string]settingsScreen {
	t.Helper()
	tr := i18n.MustNewTranslator("en")
	rights := map[string]string{"can": "", "cannot": "🚫 " + settingsText(tr, "settings_del_no_right")}
	sections := map[string][]deletionRow{"s": deletionServiceRows, "b": deletionBotRows}
	got := map[string]settingsScreen{}
	for rightName, right := range rights {
		for _, stored := range []bool{false, true} {
			state := map[bool]string{false: "none", true: "stored"}[stored]
			for code, rows := range sections {
				section, ok := settingsDelSectionByCode(code)
				if !ok {
					t.Fatalf("no section %s", code)
				}
				data := settingsDelGoldenData(stored, rows)
				prep := settingsDelPrepared{View: settingsDelPureView(tr, true, "Xenobot Test Group"), Data: data, Right: right}
				got[fmt.Sprintf("list %s %s %s", code, state, rightName)] = renderSettingsDelList(prep, section, "")
				for _, row := range rows {
					got[fmt.Sprintf("row %s %s %s", row.Code, state, rightName)] = renderSettingsDelRow(prep, section, row, "")
				}
			}
		}
	}
	return got
}

// TestSettingsDeletionPhase16ScreensUnchanged is the characterization of the Phase 16 screens: every
// list and row screen renders exactly as it did before the Member messages section existed.
func TestSettingsDeletionPhase16ScreensUnchanged(t *testing.T) {
	withRealLocales(t)
	got := settingsDelGoldenScreens(t)
	if len(got) != 2*2*(2+len(deletionServiceRows)+len(deletionBotRows)) {
		t.Fatalf("rendered %d screens, want %d", len(got), 2*2*(2+len(deletionServiceRows)+len(deletionBotRows)))
	}
	if len(settingsDelGoldenDigests) == 0 {
		names := make([]string, 0, len(got))
		for name := range got {
			names = append(names, name)
		}
		sort.Strings(names)
		var lines []string
		for _, name := range names {
			lines = append(lines, fmt.Sprintf("\t%q: %q,", name, settingsDelGoldenDigest(t, got[name])))
		}
		t.Fatalf("no recorded digests; the screens are:\n%s", strings.Join(lines, "\n"))
	}
	for name, screen := range got {
		want, ok := settingsDelGoldenDigests[name]
		if !ok {
			t.Errorf("%s: no recorded digest", name)
			continue
		}
		if digest := settingsDelGoldenDigest(t, screen); digest != want {
			keyboard, _ := json.Marshal(screen.Keyboard)
			t.Errorf("%s: the screen changed (digest %s, recorded %s)\n%s\n%s", name, digest, want, screen.Text, keyboard)
		}
	}
	for name := range settingsDelGoldenDigests {
		if _, ok := got[name]; !ok {
			t.Errorf("%s: recorded but not rendered", name)
		}
	}
}
