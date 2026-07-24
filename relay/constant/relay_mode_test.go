package constant

import "testing"

func TestPath2RelayMode(t *testing.T) {
	cases := []struct {
		path string
		want int
	}{
		{"/v1/chat/completions", RelayModeChatCompletions},
		{"/pg/chat/completions", RelayModeChatCompletions},
		{"/v1/images/generations", RelayModeImagesGenerations},
		{"/pg/images/generations", RelayModeImagesGenerations},
		{"/v1/images/edits", RelayModeImagesEdits},
		{"/v1/completions", RelayModeCompletions},
		{"/v1/embeddings", RelayModeEmbeddings},
		{"/v1/audio/speech", RelayModeAudioSpeech},
		{"/v1/audio/transcriptions", RelayModeAudioTranscription},
		{"/v1/audio/translations", RelayModeAudioTranslation},
		{"/v1/rerank", RelayModeRerank},
		{"/v1/responses", RelayModeResponses},
		{"/v1/responses/compact", RelayModeResponsesCompact},
		{"/v1/realtime", RelayModeRealtime},
		{"/v1/moderations", RelayModeModerations},
		{"/unknown/path", RelayModeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := Path2RelayMode(tc.path)
			if got != tc.want {
				t.Fatalf("Path2RelayMode(%q) = %d, want %d", tc.path, got, tc.want)
			}
		})
	}
}
