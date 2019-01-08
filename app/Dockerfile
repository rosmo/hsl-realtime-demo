FROM python:3

WORKDIR /usr/src/app

COPY requirements.txt ./
COPY templates templates
COPY static static
RUN pip install -r requirements.txt

COPY app.py ./

ENTRYPOINT [ "python" ]
CMD [ "app.py", "8080" ]