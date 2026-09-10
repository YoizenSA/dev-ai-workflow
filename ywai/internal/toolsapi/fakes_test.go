package toolsapi

import (
	"context"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

type fakeOpencodeClient struct{}

func (f *fakeOpencodeClient) Status(ctx context.Context) (opencode.ClientStatus, error) {
	return opencode.ClientStatus{Connected: false, Source: "local"}, nil
}
func (f *fakeOpencodeClient) ListModels(ctx context.Context) ([]opencode.ModelInfo, error) {
	return nil, nil
}
func (f *fakeOpencodeClient) ListAgents(ctx context.Context) ([]opencode.AgentInfo, error) {
	return nil, nil
}
func (f *fakeOpencodeClient) Sessions() opencode.SessionAPI { return nil }
