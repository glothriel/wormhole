import pymysql
import requests

from .fixtures import Client, launched_in_background


def test_mysql(executable, mysql, server):

    with launched_in_background(Client(executable, exposes=["localhost:3306"], metrics_port=8091)):
        proxied_mysql_port = int(
            requests.get(f"http://localhost:{server.admin_port}/v1/apps")
            .json()[0]["endpoint"]
            .split(":")[1]
        )

        connection = pymysql.connect(
            host=mysql.host,
            user=mysql.user,
            password=mysql.password,
            port=proxied_mysql_port,
        )
        with connection.cursor() as cursor:
            cursor.execute("CREATE DATABASE test;")
        connection.commit()
