PRJROOT=$(pwd -P);

export GOPATH=$(go env | grep GOPATH | awk -F = '{print $2}' | awk -F \" '{print $2}'):${PRJROOT};

cd ${PRJROOT}/src/ &&
go build . && \
mv ${PRJROOT}/src/src ${PRJROOT}/server && \

cd ${PRJROOT} && ./server
rm -f ${PRJROOT}/server && cd ${PRJROOT};
