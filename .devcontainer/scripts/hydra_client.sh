# script to create a local hydra client with a random secret

CLIENTID="governor-dev"
SECRET=$(openssl rand -hex 16)

echo "Creating hydra client-id ${CLIENTID} and client-secret ${SECRET}"

hydra clients create \
    --audience http://api:3001/ \
    --id ${CLIENTID} \
    --secret ${SECRET} \
    --grant-types client_credentials \
    --response-types token,code \
    --token-endpoint-auth-method client_secret_post \
    --scope write,read

cat <<EOF
Your client "${CLIENTID}" was generated with secret "${SECRET}"
You can fetch a JWT token like so:

hydra token client \\
  --audience http://api:3001/ \\
  --client-id ${CLIENTID} \\
  --client-secret ${SECRET} \\
  --scope write,read

EOF
