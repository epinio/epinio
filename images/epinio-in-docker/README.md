Build this image:

```
docker build -t epinio-in-docker .
```

Run it:

```
docker run --name epinio-in-docker  -it --privileged -p 443:443 epinio-in-docker
```

Visit https://epinio.127.0.0.1.sslip.io in your browser
