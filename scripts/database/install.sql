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