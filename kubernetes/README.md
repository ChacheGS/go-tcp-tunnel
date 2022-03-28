# Kubernertes helm charts

## Getting started

```sh
# Create certificate
../hack/certutil.sh cert --addr <YOUR_PUBLIC_HOSTNAME_OR_IP>

# Install certificate as secret into the public server cluster
../hack/certutil.sh secret --namespace tcptunnel --install

# Install server
helm upgrade --namespace tcptunnel --create-namespace --install tcptunnel ./chart
```