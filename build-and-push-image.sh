
if [ -z $1 ] || [ "$1" == "-h" ]; then
  echo "usage: ./build-image.sh <tag (eg: 1.2.3)>"
  exit 0
fi

docker buildx build --platform linux/arm64,linux/amd64 -t scottyjoe9/dhcrelay-dhcpeeper:$1 .

docker push scottyjoe9/dhcrelay-dhcpeeper:$1
