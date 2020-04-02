go get github.com/boltdb/bolt
go get github.com/didip/tollbooth
go get github.com/gorilla/mux
go get github.com/skip2/go-qrcode
go get github.com/tidwall/gjson

PRJROOT=$(pwd -P);

export GOPATH=$(go env | grep GOPATH | awk -F = '{print $2}' | awk -F \" '{print $2}'):${PRJROOT};

cd ${PRJROOT}/src/ &&
go build . && \
mv ${PRJROOT}/src/src ${PRJROOT}/server && \

cd ${PRJROOT} && ./server
rm -f ${PRJROOT}/server && cd ${PRJROOT};
