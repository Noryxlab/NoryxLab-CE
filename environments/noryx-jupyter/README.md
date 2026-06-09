# Noryx Jupyter environment

System workspace image for Jupyter:

```bash
docker build -t harbor.example.local/noryx-environments/noryx-jupyter:0.1.0 -f environments/noryx-jupyter/Dockerfile .
docker push harbor.example.local/noryx-environments/noryx-jupyter:0.1.0
```

Custom Jupyter environments should inherit from this image:

```dockerfile
FROM harbor.example.local/noryx-environments/noryx-jupyter:0.1.0
RUN python3 -m pip install --no-cache-dir pandas
```
