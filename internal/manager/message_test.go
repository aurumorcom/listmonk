package manager

import (
	"fmt"
	"sync"
	"testing"

	"github.com/knadh/listmonk/models"
)

func TestScopeContextIsolationConcurrent(t *testing.T) {
	const count = 10
	var wg sync.WaitGroup

	for i := 1; i <= count; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			companyName := fmt.Sprintf("Company_%d", id)
			userName := fmt.Sprintf("User_%d", id)

			attribs := models.JSON{
				"context": map[string]any{"company": companyName},
				"user":    map[string]any{"name": userName},
			}

			sub := models.Subscriber{
				Base:    models.Base{ID: id},
				Email:   fmt.Sprintf("user%d@example.com", id),
				Attribs: attribs,
			}

			scope := ExtractTemplateScope(sub)

			ctxMap, ok := scope["Context"].(map[string]any)
			if !ok || ctxMap["company"] != companyName {
				t.Errorf("worker %d: expected company %s, got %v", id, companyName, scope["Context"])
			}

			userMap, ok := scope["User"].(map[string]any)
			if !ok || userMap["name"] != userName {
				t.Errorf("worker %d: expected user name %s, got %v", id, userName, scope["User"])
			}
		}(i)
	}

	wg.Wait()
}
