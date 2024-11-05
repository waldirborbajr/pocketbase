package apis_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"gocloud.dev/blob"
)

func TestBackupsList(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodGet,
			URL:    "/api/backups",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodGet,
			URL:    "/api/backups",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.ZT3F0Z3iM-xbGgSG3LEKiEzHrPHr8t8IuHLZGGNuxLo",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (empty list)",
			Method: http.MethodGet,
			URL:    "/api/backups",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`[]`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser",
			Method: http.MethodGet,
			URL:    "/api/backups",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"test1.zip"`,
				`"test2.zip"`,
				`"test3.zip"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestBackupsCreate(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodPost,
			URL:    "/api/backups",
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodPost,
			URL:    "/api/backups",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.ZT3F0Z3iM-xbGgSG3LEKiEzHrPHr8t8IuHLZGGNuxLo",
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (pending backup)",
			Method: http.MethodPost,
			URL:    "/api/backups",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Store().Set(core.StoreKeyActiveBackup, "")
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (autogenerated name)",
			Method: http.MethodPost,
			URL:    "/api/backups",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				files, err := getBackupFiles(app)
				if err != nil {
					t.Fatal(err)
				}

				if total := len(files); total != 1 {
					t.Fatalf("Expected 1 backup file, got %d", total)
				}

				expected := "pb_backup_"
				if !strings.HasPrefix(files[0].Key, expected) {
					t.Fatalf("Expected backup file with prefix %q, got %q", expected, files[0].Key)
				}
			},
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{
				"*":              0,
				"OnBackupCreate": 1,
			},
		},
		{
			Name:   "authorized as superuser (invalid name)",
			Method: http.MethodPost,
			URL:    "/api/backups",
			Body:   strings.NewReader(`{"name":"!test.zip"}`),
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"data":{`,
				`"name":{"code":"validation_match_invalid"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (valid name)",
			Method: http.MethodPost,
			URL:    "/api/backups",
			Body:   strings.NewReader(`{"name":"test.zip"}`),
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				files, err := getBackupFiles(app)
				if err != nil {
					t.Fatal(err)
				}

				if total := len(files); total != 1 {
					t.Fatalf("Expected 1 backup file, got %d", total)
				}

				expected := "test.zip"
				if files[0].Key != expected {
					t.Fatalf("Expected backup file %q, got %q", expected, files[0].Key)
				}
			},
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{
				"*":              0,
				"OnBackupCreate": 1,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestBackupUpload(t *testing.T) {
	t.Parallel()

	// create dummy form data bodies
	type body struct {
		buffer      io.Reader
		contentType string
	}
	bodies := make([]body, 10)
	for i := 0; i < 10; i++ {
		func() {
			zb := new(bytes.Buffer)
			zw := zip.NewWriter(zb)
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}

			b := new(bytes.Buffer)
			mw := multipart.NewWriter(b)

			mfw, err := mw.CreateFormFile("file", "test")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(mfw, zb); err != nil {
				t.Fatal(err)
			}

			mw.Close()

			bodies[i] = body{
				buffer:      b,
				contentType: mw.FormDataContentType(),
			}
		}()
	}
	// ---

	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodPost,
			URL:    "/api/backups/upload",
			Body:   bodies[0].buffer,
			Headers: map[string]string{
				"Content-Type": bodies[0].contentType,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodPost,
			URL:    "/api/backups/upload",
			Body:   bodies[1].buffer,
			Headers: map[string]string{
				"Content-Type":  bodies[1].contentType,
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.ZT3F0Z3iM-xbGgSG3LEKiEzHrPHr8t8IuHLZGGNuxLo",
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (missing file)",
			Method: http.MethodPost,
			URL:    "/api/backups/upload",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				ensureNoBackups(t, app)
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (existing backup name)",
			Method: http.MethodPost,
			URL:    "/api/backups/upload",
			Body:   bodies[3].buffer,
			Headers: map[string]string{
				"Content-Type":  bodies[3].contentType,
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				fsys, err := app.NewBackupsFilesystem()
				if err != nil {
					t.Fatal(err)
				}
				defer fsys.Close()
				// create a dummy backup file to simulate existing backups
				if err := fsys.Upload([]byte("123"), "test"); err != nil {
					t.Fatal(err)
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				files, _ := getBackupFiles(app)
				if total := len(files); total != 1 {
					t.Fatalf("Expected %d backup file, got %d", 1, total)
				}
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{"file":{`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (valid file)",
			Method: http.MethodPost,
			URL:    "/api/backups/upload",
			Body:   bodies[4].buffer,
			Headers: map[string]string{
				"Content-Type":  bodies[4].contentType,
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				files, _ := getBackupFiles(app)
				if total := len(files); total != 1 {
					t.Fatalf("Expected %d backup file, got %d", 1, total)
				}
			},
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestBackupsDownload(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodGet,
			URL:    "/api/backups/test1.zip",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "with record auth header",
			Method: http.MethodGet,
			URL:    "/api/backups/test1.zip",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.ZT3F0Z3iM-xbGgSG3LEKiEzHrPHr8t8IuHLZGGNuxLo",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "with superuser auth header",
			Method: http.MethodGet,
			URL:    "/api/backups/test1.zip",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "with empty or invalid token",
			Method: http.MethodGet,
			URL:    "/api/backups/test1.zip?token=",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "with valid record auth token",
			Method: http.MethodGet,
			URL:    "/api/backups/test1.zip?token=eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.ZT3F0Z3iM-xbGgSG3LEKiEzHrPHr8t8IuHLZGGNuxLo",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "with valid record file token",
			Method: http.MethodGet,
			URL:    "/api/backups/test1.zip?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsImV4cCI6MjUyNDYwNDQ2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJfcGJfdXNlcnNfYXV0aF8ifQ.nSTLuCPcGpWn2K2l-BFkC3Vlzc-ZTDPByYq8dN1oPSo",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "with valid superuser auth token",
			Method: http.MethodGet,
			URL:    "/api/backups/test1.zip?token=eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "with expired superuser file token",
			Method: http.MethodGet,
			URL:    "/api/backups/test1.zip?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsImV4cCI6MTY0MDk5MTY2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJwYmNfMzE0MjYzNTgyMyJ9.nqqtqpPhxU0045F4XP_ruAkzAidYBc5oPy9ErN3XBq0",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "with valid superuser file token but missing backup name",
			Method: http.MethodGet,
			URL:    "/api/backups/missing?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsImV4cCI6MjUyNDYwNDQ2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJwYmNfMzE0MjYzNTgyMyJ9.Lupz541xRvrktwkrl55p5pPCF77T69ZRsohsIcb2dxc",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "with valid superuser file token",
			Method: http.MethodGet,
			URL:    "/api/backups/test1.zip?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsImV4cCI6MjUyNDYwNDQ2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJwYmNfMzE0MjYzNTgyMyJ9.Lupz541xRvrktwkrl55p5pPCF77T69ZRsohsIcb2dxc",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				"storage/",
				"data.db",
				"auxiliary.db",
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:   "with valid superuser file token and backup name with escaped char",
			Method: http.MethodGet,
			URL:    "/api/backups/%40test4.zip?token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsImV4cCI6MjUyNDYwNDQ2MSwidHlwZSI6ImZpbGUiLCJjb2xsZWN0aW9uSWQiOiJwYmNfMzE0MjYzNTgyMyJ9.Lupz541xRvrktwkrl55p5pPCF77T69ZRsohsIcb2dxc",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				"storage/",
				"data.db",
				"auxiliary.db",
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestBackupsDelete(t *testing.T) {
	t.Parallel()

	noTestBackupFilesChanges := func(t testing.TB, app *tests.TestApp) {
		files, err := getBackupFiles(app)
		if err != nil {
			t.Fatal(err)
		}

		expected := 4
		if total := len(files); total != expected {
			t.Fatalf("Expected %d backup(s), got %d", expected, total)
		}
	}

	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodDelete,
			URL:    "/api/backups/test1.zip",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				noTestBackupFilesChanges(t, app)
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodDelete,
			URL:    "/api/backups/test1.zip",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.ZT3F0Z3iM-xbGgSG3LEKiEzHrPHr8t8IuHLZGGNuxLo",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				noTestBackupFilesChanges(t, app)
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (missing file)",
			Method: http.MethodDelete,
			URL:    "/api/backups/missing.zip",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				noTestBackupFilesChanges(t, app)
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (existing file with matching active backup)",
			Method: http.MethodDelete,
			URL:    "/api/backups/test1.zip",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}

				// mock active backup with the same name to delete
				app.Store().Set(core.StoreKeyActiveBackup, "test1.zip")
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				noTestBackupFilesChanges(t, app)
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (existing file and no matching active backup)",
			Method: http.MethodDelete,
			URL:    "/api/backups/test1.zip",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}

				// mock active backup with different name
				app.Store().Set(core.StoreKeyActiveBackup, "new.zip")
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				files, err := getBackupFiles(app)
				if err != nil {
					t.Fatal(err)
				}

				if total := len(files); total != 3 {
					t.Fatalf("Expected %d backup files, got %d", 3, total)
				}

				deletedFile := "test1.zip"

				for _, f := range files {
					if f.Key == deletedFile {
						t.Fatalf("Expected backup %q to be deleted", deletedFile)
					}
				}
			},
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (backup with escaped character)",
			Method: http.MethodDelete,
			URL:    "/api/backups/%40test4.zip",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				files, err := getBackupFiles(app)
				if err != nil {
					t.Fatal(err)
				}

				if total := len(files); total != 3 {
					t.Fatalf("Expected %d backup files, got %d", 3, total)
				}

				deletedFile := "@test4.zip"

				for _, f := range files {
					if f.Key == deletedFile {
						t.Fatalf("Expected backup %q to be deleted", deletedFile)
					}
				}
			},
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestBackupsRestore(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:   "unauthorized",
			Method: http.MethodPost,
			URL:    "/api/backups/test1.zip/restore",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodPost,
			URL:    "/api/backups/test1.zip/restore",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.ZT3F0Z3iM-xbGgSG3LEKiEzHrPHr8t8IuHLZGGNuxLo",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (missing file)",
			Method: http.MethodPost,
			URL:    "/api/backups/missing.zip/restore",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser (active backup process)",
			Method: http.MethodPost,
			URL:    "/api/backups/test1.zip/restore",
			Headers: map[string]string{
				"Authorization": "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := createTestBackups(app); err != nil {
					t.Fatal(err)
				}

				app.Store().Set(core.StoreKeyActiveBackup, "")
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

// -------------------------------------------------------------------

func createTestBackups(app core.App) error {
	ctx := context.Background()

	if err := app.CreateBackup(ctx, "test1.zip"); err != nil {
		return err
	}

	if err := app.CreateBackup(ctx, "test2.zip"); err != nil {
		return err
	}

	if err := app.CreateBackup(ctx, "test3.zip"); err != nil {
		return err
	}

	if err := app.CreateBackup(ctx, "@test4.zip"); err != nil {
		return err
	}

	return nil
}

func getBackupFiles(app core.App) ([]*blob.ListObject, error) {
	fsys, err := app.NewBackupsFilesystem()
	if err != nil {
		return nil, err
	}
	defer fsys.Close()

	return fsys.List("")
}

func ensureNoBackups(t testing.TB, app *tests.TestApp) {
	files, err := getBackupFiles(app)
	if err != nil {
		t.Fatal(err)
	}

	if total := len(files); total != 0 {
		t.Fatalf("Expected 0 backup files, got %d", total)
	}
}
