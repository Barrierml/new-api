package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestConvertImageEditRequestMultipart verifies that ConvertImageRequest
// re-serializes multipart image edit requests with all fields (including
// stream) and the file intact, both when the form was already parsed and when
// it must be re-parsed from the reusable body.
func TestConvertImageEditRequestMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newMultipartContext := func(t *testing.T, prompt string) *gin.Context {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("model", "gpt-image-1"))
		require.NoError(t, writer.WriteField("prompt", prompt))
		require.NoError(t, writer.WriteField("stream", "true"))
		require.NoError(t, writer.WriteField("partial_images", "3"))
		part, err := writer.CreateFormFile("image", "input.png")
		require.NoError(t, err)
		_, err = part.Write([]byte("fake image"))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return c
	}

	convertAndReplay := func(t *testing.T, c *gin.Context, prompt string) {
		info := &relaycommon.RelayInfo{
			RelayMode: relayconstant.RelayModeImagesEdits,
		}
		request := dto.ImageRequest{
			Model:  "gpt-image-1",
			Prompt: prompt,
			Stream: common.GetPointer(true),
		}

		converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
		require.NoError(t, err)
		convertedBody, ok := converted.(*bytes.Buffer)
		require.True(t, ok)

		replayedRequest := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(convertedBody.Bytes()))
		replayedRequest.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
		require.NoError(t, replayedRequest.ParseMultipartForm(32<<20))

		require.Equal(t, "gpt-image-1", replayedRequest.PostForm.Get("model"))
		require.Equal(t, prompt, replayedRequest.PostForm.Get("prompt"))
		require.Equal(t, "true", replayedRequest.PostForm.Get("stream"))
		require.Equal(t, "3", replayedRequest.PostForm.Get("partial_images"))
		require.Len(t, replayedRequest.MultipartForm.File["image"], 1)

		file, err := replayedRequest.MultipartForm.File["image"][0].Open()
		require.NoError(t, err)
		defer file.Close()
		fileBytes, err := io.ReadAll(file)
		require.NoError(t, err)
		require.Equal(t, []byte("fake image"), fileBytes)
	}

	t.Run("with pre-parsed form", func(t *testing.T) {
		prompt := "edit this image"
		c := newMultipartContext(t, prompt)
		require.NoError(t, c.Request.ParseMultipartForm(32<<20))

		convertAndReplay(t, c, prompt)
	})

	t.Run("re-parses reusable body when form is missing", func(t *testing.T) {
		prompt := "edit without pre-parsed form"
		c := newMultipartContext(t, prompt)

		storage, err := common.GetBodyStorage(c)
		require.NoError(t, err)
		c.Request.Body = io.NopCloser(storage)
		c.Request.MultipartForm = nil
		c.Request.PostForm = nil

		convertAndReplay(t, c, prompt)
	})
}

// TestConvertImageGenerationsWithReferenceImageReroutesToEdits 验证 playground
// 图生图场景:JSON 请求带 image 参考图时,generations 请求被改打到
// /images/edits(OpenAI generations 端点不认 image 字段);不带 image 或多
// part 请求路径不变。
func TestConvertImageGenerationsWithReferenceImageReroutesToEdits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	a := &Adaptor{}

	newJSONContext := func(t *testing.T) *gin.Context {
		req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = req
		return c
	}

	// 带参考图 → 路径改打 edits
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeImagesGenerations,
		RequestURLPath: "/v1/images/generations",
	}
	req := dto.ImageRequest{
		Model:  "gpt-image-2",
		Prompt: "edit it",
		Image:  json.RawMessage(`"data:image/png;base64,AAA"`),
	}
	converted, err := a.ConvertImageRequest(newJSONContext(t), info, req)
	require.NoError(t, err)
	require.Equal(t, "/v1/images/edits", info.RequestURLPath)
	got, ok := converted.(dto.ImageRequest)
	require.True(t, ok)
	require.Equal(t, "gpt-image-2", got.Model)

	// 不带参考图 → 路径保持 generations
	info2 := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeImagesGenerations,
		RequestURLPath: "/v1/images/generations",
	}
	_, err = a.ConvertImageRequest(newJSONContext(t), info2, dto.ImageRequest{Model: "gpt-image-2", Prompt: "draw"})
	require.NoError(t, err)
	require.Equal(t, "/v1/images/generations", info2.RequestURLPath)
}
