Directory structure:
└── GoFetch-downloader-GoFetch/
    ├── README.md
    ├── analyze.py
    ├── benchmark.py
    ├── go.mod
    ├── go.sum
    ├── LICENSE
    ├── main.go
    ├── TODO.md
    ├── .goreleaser.yaml
    ├── assets/
    │   └── demo.tape
    ├── cmd/
    │   ├── add.go
    │   ├── autoresume_test.go
    │   ├── cli_test.go
    │   ├── cmd_test.go
    │   ├── get_test.go
    │   ├── http_handler_test.go
    │   ├── lock.go
    │   ├── lock_test.go
    │   ├── ls.go
    │   ├── mirrors_integration_test.go
    │   ├── pause.go
    │   ├── resume.go
    │   ├── rm.go
    │   ├── root.go
    │   ├── server.go
    │   ├── startup_test.go
    │   └── utils.go
    ├── extension-chrome/
    │   ├── background.js
    │   ├── manifest.json
    │   ├── popup.css
    │   ├── popup.html
    │   └── popup.js
    ├── extension-firefox/
    │   ├── background.js
    │   ├── extension.zip
    │   ├── manifest.json
    │   ├── popup.css
    │   ├── popup.html
    │   ├── popup.js
    │   └── STORE_DESCRIPTION.md
    ├── internal/
    │   ├── benchmark/
    │   │   ├── metrics.go
    │   │   └── metrics_test.go
    │   ├── clipboard/
    │   │   └── validator.go
    │   ├── config/
    │   │   ├── paths.go
    │   │   ├── paths_test.go
    │   │   ├── settings.go
    │   │   └── settings_test.go
    │   ├── download/
    │   │   ├── filename_test.go
    │   │   ├── manager.go
    │   │   ├── manager_test.go
    │   │   ├── mirror_resume_test.go
    │   │   ├── pool.go
    │   │   ├── pool_status_test.go
    │   │   ├── pool_test.go
    │   │   └── resume_test.go
    │   ├── engine/
    │   │   ├── probe.go
    │   │   ├── concurrent/
    │   │   │   ├── chunk_test.go
    │   │   │   ├── concurrent_test.go
    │   │   │   ├── downloader.go
    │   │   │   ├── health.go
    │   │   │   ├── health_test.go
    │   │   │   ├── mirrors_test.go
    │   │   │   ├── sequential_test.go
    │   │   │   ├── switch_429_test.go
    │   │   │   ├── task.go
    │   │   │   ├── task_queue.go
    │   │   │   ├── task_queue_test.go
    │   │   │   ├── task_test.go
    │   │   │   └── worker.go
    │   │   ├── events/
    │   │   │   ├── events.go
    │   │   │   └── events_test.go
    │   │   ├── single/
    │   │   │   ├── downloader.go
    │   │   │   └── downloader_test.go
    │   │   ├── state/
    │   │   │   ├── db.go
    │   │   │   ├── db_test.go
    │   │   │   ├── state.go
    │   │   │   └── state_test.go
    │   │   └── types/
    │   │       ├── accuracy_test.go
    │   │       ├── config.go
    │   │       ├── config_test.go
    │   │       ├── errors.go
    │   │       ├── models.go
    │   │       ├── progress.go
    │   │       └── progress_test.go
    │   ├── testutil/
    │   │   ├── mock_server.go
    │   │   ├── mock_server_test.go
    │   │   └── test_files.go
    │   ├── tui/
    │   │   ├── autoresume_test.go
    │   │   ├── config.go
    │   │   ├── constants.go
    │   │   ├── gradient.go
    │   │   ├── graph.go
    │   │   ├── keys.go
    │   │   ├── list.go
    │   │   ├── model.go
    │   │   ├── path_test.go
    │   │   ├── polling_test.go
    │   │   ├── reporter.go
    │   │   ├── resume_lifecycle_test.go
    │   │   ├── settings_view.go
    │   │   ├── startup_test.go
    │   │   ├── styles.go
    │   │   ├── update.go
    │   │   ├── update_test.go
    │   │   ├── view.go
    │   │   ├── colors/
    │   │   │   └── colors.go
    │   │   └── components/
    │   │       ├── box.go
    │   │       ├── chunkmap.go
    │   │       ├── chunkmap_test.go
    │   │       ├── confirmation_modal.go
    │   │       ├── filepicker_modal.go
    │   │       ├── status.go
    │   │       └── tab_bar.go
    │   ├── utils/
    │   │   ├── debug.go
    │   │   ├── debug_test.go
    │   │   ├── filename.go
    │   │   ├── filename_test.go
    │   │   ├── path.go
    │   │   ├── path_test.go
    │   │   ├── size_converter.go
    │   │   └── size_converter_test.go
    │   └── version/
    │       └── version.go
    └── .github/
        ├── ISSUE_TEMPLATE/
        │   ├── bug_report.md
        │   └── feature_request.md
        └── workflows/
            ├── benchmark.yml
            └── build.yml

