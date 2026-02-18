File Structure
==============

Repository Layout
-----------------
```
GoFetch/
├── cmd/
│   └── gofetch/
│       └── main.go              # CLI entrypoint
├── internal/
│   ├── cli/                     # Cobra commands + CLI glue
│   │   ├── add.go
│   │   ├── lock.go
│   │   ├── ls.go
│   │   ├── pause.go
│   │   ├── resume.go
│   │   ├── rm.go
│   │   ├── root.go
│   │   ├── server.go
│   │   ├── shutdown.go
│   │   ├── token.go
│   │   └── utils.go
│   ├── config/                  # runtime config + paths
│   ├── core/                    # service wiring & interfaces
│   ├── download/                # download engine
│   │   ├── manager.go
│   │   ├── pool.go
│   │   ├── concurrent/
│   │   ├── single/
│   │   ├── messages/
│   │   └── types/
│   ├── events/                  # event stream definitions
│   ├── state/                   # persistence, db
│   ├── utils/                   # internal helpers
│   ├── clipboard/
│   └── probe.go                 # server probe logic
├── docs/
│   ├── ARCHITECTURE.md
│   ├── DOCS.md
│   └── FILE_STRUCTURE.md
├── go.mod
├── go.sum
├── README.md
└── benchmark.py
```

Guiding Principles
------------------
- Keep CLI code isolated in internal/cli to avoid polluting core logic.
- Treat internal/download as the engine; it should stay independent of CLI UX.
- Use internal/events and internal/state for progress and persistence concerns.
- Only cmd/gofetch should be a public entrypoint binary.
