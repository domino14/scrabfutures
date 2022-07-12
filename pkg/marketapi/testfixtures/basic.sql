-- a basic fixture with some data...

INSERT INTO users(id, username, email, password)
values
    (1, "cesar", "delsolar@gmail.com", "foo"),
    (2, "josh", "josh@gmail.com", "foo");

INSERT INTO portfolios(user_id, tokens)
values
    (1, 2000),
    (2, 2000);

INSERT INTO markets(id, uuid, description, date_created, is_open)
values
    (1, "nationals2022", "Nationals 2022", "2022-07-08T14:00:00Z", 0);

INSERT INTO securities(uuid, description, shortname, date_created, market_id, shares_outstanding, last_price)
values
    ("S1uuid", "Kenji wins nationals", "KNJI", "2022-07-08T14:00:00Z", 1, 0, 25),
    ("S2uuid", "Noah wins nationals", "NOAH", "2022-07-08T14:00:01Z", 1, 0, 25),
    ("S3uuid", "César wins nationals", "CSAR", "2022-07-08T14:00:01Z", 1, 0, 25),
    ("S4uuid", "Josh wins nationals", "JOSH", "2022-07-08T14:00:01Z", 1, 0, 25);

