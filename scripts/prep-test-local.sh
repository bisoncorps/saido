cat <<EOF > config-test.yaml
hosts:
  children:
    "127.0.0.1":
      connection:
        type: local

metrics:
- memory
- cpu
EOF

