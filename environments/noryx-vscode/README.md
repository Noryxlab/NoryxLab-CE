# Noryx VSCode environment

System workspace image for VSCode:

```bash
docker build -t harbor.example.local/noryx-environments/noryx-vscode:0.1.1 -f environments/noryx-vscode/Dockerfile .
docker push harbor.example.local/noryx-environments/noryx-vscode:0.1.1
```

Custom VSCode environments should inherit from this image:

```dockerfile
FROM harbor.example.local/noryx-environments/noryx-vscode:0.1.1
RUN python3 -m pip install --no-cache-dir pandas
```
