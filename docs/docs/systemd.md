# Running _Ociregistry_ as a systemd Service

The `systemd-service` directory has a systemd unit file for running the server as a systemd service:

```shell
[Unit]
Description=ociregistry
Documentation=https://github.com/aceeric/ociregistry
After=network.target

[Service]
ExecStart=/bin/ociregistry-server\
  --image-path=/var/lib/ociregistry\
  --log-level=info\
  serve
Type=simple
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

You can use the provided `manual-install` script in that directory to perform the installation.
