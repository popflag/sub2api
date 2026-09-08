package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestDefaultCodexInstructionsSetting(t *testing.T) {
	ctx := context.Background()
	var absent *SettingService
	require.False(t, absent.IsOpenAIDefaultCodexInstructionsDisabled(ctx))
	for _, value := range []string{"", "false", "true", "invalid"} {
		t.Run(value, func(t *testing.T) {
			resetGatewayForwardingSettingsCacheForTest(t)
			values := map[string]string{SettingKeyOpenAIDisableDefaultCodexInstructions: value}
			svc := NewSettingService(&gatewayTTLSettingRepo{data: values}, &config.Config{})
			require.Equal(t, value == "true", svc.parseSettings(values).OpenAIDisableDefaultCodexInstructions)
			// Read twice to cover both the repository and cached paths.
			for range 2 {
				require.Equal(t, value == "true", svc.IsOpenAIDefaultCodexInstructionsDisabled(ctx))
			}
		})
	}
}

func TestDefaultCodexInstructionsSettingUpdateRefreshesCache(t *testing.T) {
	resetGatewayForwardingSettingsCacheForTest(t)
	repo := &gatewayTTLSettingRepo{}
	svc := NewSettingService(repo, &config.Config{})
	for _, disabled := range []bool{true, false} {
		require.NoError(t, svc.UpdateSettings(context.Background(), &SystemSettings{
			OpenAIDisableDefaultCodexInstructions: disabled,
		}))
		require.Equal(t, strconv.FormatBool(disabled), repo.data[SettingKeyOpenAIDisableDefaultCodexInstructions])
		require.Equal(t, disabled, svc.IsOpenAIDefaultCodexInstructionsDisabled(context.Background()))
	}
}

func TestForwardDefaultCodexInstructionsSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, chat := range []bool{false, true} {
		for _, tc := range []struct {
			name     string
			disabled bool
			fields   string
			want     string
		}{
			{name: "default", want: defaultCodexSynthInstructions("gpt-5.5")},
			{name: "disabled", disabled: true},
			{name: "blank", disabled: true, fields: `"instructions":"   ",`, want: "   "},
			{name: "explicit", disabled: true, fields: `"instructions":"client guidance",`, want: "client guidance"},
			{name: "system", disabled: true, fields: `"input":[{"role":"system","content":"system guidance"}],`, want: "system guidance"},
		} {
			t.Run(tc.name+"/chat="+strconv.FormatBool(chat), func(t *testing.T) {
				resetGatewayForwardingSettingsCacheForTest(t)
				upstream := &httpUpstreamRecorder{resp: &http.Response{
					StatusCode: http.StatusBadRequest,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"stop after capture"}}`)),
				}}
				cfg := &config.Config{}
				svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream,
					settingService: NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{
						SettingKeyOpenAIDisableDefaultCodexInstructions: strconv.FormatBool(tc.disabled),
					}}, cfg),
				}
				input := `"input":[{"role":"user","content":"hello"}],`
				if tc.name == "system" {
					input = ""
				}
				body := []byte(`{` + tc.fields + input + `"model":"gpt-5.5","stream":true}`)
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
				c.Request.Header.Set("Content-Type", "application/json")
				var err error
				if chat {
					_, err = svc.ForwardAsChatCompletions(context.Background(), c, compatCyberOAuthAccount(), body, "", "gpt-5.5")
				} else {
					_, err = svc.Forward(context.Background(), c, compatCyberOAuthAccount(), body)
				}
				require.Error(t, err)
				require.NotNil(t, upstream.lastReq)
				instructions := gjson.GetBytes(upstream.lastBody, "instructions")
				require.True(t, instructions.Exists())
				require.Equal(t, tc.want, instructions.String())
			})
		}
	}
}
