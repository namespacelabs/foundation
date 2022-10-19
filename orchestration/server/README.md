Releasing a new orchestrator:

```
nsdev build orchestration/server/ --build_platforms=linux/amd64,linux/arm64  --base_repository=us-docker.pkg.dev/foundation-344819/prebuilts
```

And copy the corresponding digest to internal/service/versions/tool/versions.json.