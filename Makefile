deploy-ingressController:
	kubectl -n ingress-nginx apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v0.41.2/deploy/static/provider/cloud/deploy.yaml

deploy-example: deploy-certmanager deploy-clusterIssuer deploy-cert deploy-app deploy-app2

deploy-certmanager:
	# Cert manager, service and certificates are in the same namespace "cert-manager".
	# todo - lookup how to change the cert-manager namespace to a common one like apim-lrt-application
	# Issuer is in the common namespace
	kubectl create ns cert-manager


	helm install cert-manager --namespace cert-manager jetstack/cert-manager \
		--version v1.0.1 --set installCRDs=true

deploy-clusterIssuer:
	kubectl create ns issuerns
	kubectl -n issuerns apply -f issuer.yaml

# create deployment, svc
deploy-cert:
	kubectl create ns application
	kubectl -n application apply -f cert.yaml

deploy-app:
	kubectl -n application apply -f app.yaml
	kubectl -n application apply -f ingress.yaml

deploy-app2:
	kubectl create ns application2
	kubectl -n application2 apply -f cert2.yaml
	kubectl -n application2 apply -f app.yaml
	kubectl -n application2 apply -f ingress2.yaml

clean-service:
	kubectl -n application delete ingress example-app
	kubectl -n application delete deployment example-deploy
	kubectl -n application delete service example-service
	kubectl -n application delete certificate selfsigned-cert
	kubectl -n application delete secret selfsigned-cert-tls




