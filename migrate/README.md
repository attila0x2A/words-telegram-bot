From the root of the project:

```shell
sudo docker build -t loader . -f migrate/Dockerfile
sudo docker run --rm --name loader --mount source=words-vol,target=/words-vol/db/ loader
```

To save image:

```shell
sudo docker save loader -o loader_image.tar
sudo chmod +r loader_image.tar
```


Test image changes:
```shell
sudo docker run --rm --name temp_bash --mount source=words-vol,target=/words-vol/db/ -it ubuntu /bin/bash
```
