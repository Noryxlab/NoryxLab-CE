# Noryx RStudio environment

System workspace image aligned with the Onyxia RStudio image lineage:

```bash
docker build -t harbor.lan/noryx-environments/noryx-rstudio:0.1.0 -f environments/noryx-rstudio/Dockerfile .
docker push harbor.lan/noryx-environments/noryx-rstudio:0.1.0
```

Custom RStudio environments should inherit from this image:

```dockerfile
FROM harbor.lan/noryx-environments/noryx-rstudio:0.1.0
RUN R -q -e 'install.packages("dplyr")'
```
