-- please note that the table name is "per corpus"
-- and the name should follow the template: `[corpus_name]_scoll_query`
CREATE TABLE scoll_query (
    id int auto_increment,
    lemma varchar(60),
    p_lemma varchar(60),
    upos varchar(60),
    p_upos varchar(60),
    deprel varchar(60),
    result TEXT,
    result_type varchar(60),
    PRIMARY KEY (id)
);

-- please note that the table name is "per corpus"
-- and the name should follow the template: `[corpus_name]_scoll_fcrit`
CREATE TABLE scoll_fcrit (
    id int auto_increment,
    scoll_query_id INT NOT NULL,
    attr varchar(60),
    result TEXT,
    result_type varchar(60),
    PRIMARY KEY (id),
    FOREIGN KEY (scoll_query_id) REFERENCES scoll_query(id)
);

CREATE TABLE fcolls (
    id int auto_increment,
    lemma varchar(100),
    upos varchar(40),
    p_lemma varchar(100),
    p_upos varchar(40),
    deprel varchar(40),
    freq int,
    PRIMARY KEY (id)
);

CREATE TABLE mquery_load_log (
    id int auto_increment NOT NULL,
    worker_id varchar(8) NOT NULL,
    start_dt datetime NOT NULL,
    end_dt datetime NOT NULL,
    func varchar(60) NOT NULL,
    err text,
    PRIMARY KEY (id)
);


CREATE TABLE mquery_load_timeline (
    dt datetime NOT NULL,
    wload float NOT NULL,
    worker_id varchar(60) NOT NULL,
    PRIMARY KEY (worker_id, dt)
);