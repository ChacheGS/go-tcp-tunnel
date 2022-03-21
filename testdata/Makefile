HOSTNAME ?= localhost

.PHONY: cert
cert:
	echo "subjectAltName = DNS:$(HOSTNAME)" > san.txt
	openssl genrsa -out tls.key
	openssl req -new -key tls.key -out tls.csr -subj "/CN=$(HOSTNAME)"
	openssl x509 -in tls.csr -out tls.crt -req -signkey tls.key -days 36500 -extfile san.txt
