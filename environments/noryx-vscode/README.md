# Noryx VSCode environment

System workspace image for VSCode:

```bash
docker build -t harbor.lan/noryx-environments/noryx-vscode:0.1.0 -f environments/noryx-vscode/Dockerfile .
docker push harbor.lan/noryx-environments/noryx-vscode:0.1.0
```

Custom VSCode environments should inherit from this image:

```dockerfile
FROM harbor.lan/noryx-environments/noryx-vscode:0.1.0
RUN python3 -m pip install --no-cache-dir pandas
```
