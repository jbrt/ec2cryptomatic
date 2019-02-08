FROM python:3.6.8-alpine

LABEL maintainer="julien@toshokan.fr"

COPY . /app
WORKDIR /app
RUN pip install -r requirements.txt

ENTRYPOINT ["/usr/local/bin/python3", "/app/ec2cryptomatic.py"]
