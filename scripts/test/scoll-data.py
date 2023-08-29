import argparse
import json
import urllib.request
import os

WORDS_PATH = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'words.txt'))

if __name__ == "__main__":
    argparser = argparse.ArgumentParser('Scoll test script')
    argparser.add_argument('--conf', dest='conf_path', action='store', help='Path to MQuery conf file')
    argparser.add_argument('--words', dest='words_path', action='store', default=WORDS_PATH, help='Path to text file with tested words')
    argparser.add_argument('--corpname', dest='corpname', action='store', help='Corpus used for tests')
    argparser.add_argument('--server', dest='server_path', action='store', help='Server base url')
    argparser.add_argument('--out', dest='out_path', action='store', help='Output file path')
    args = argparser.parse_args()

    with open(args.conf_path) as f:
        mquery_conf = json.load(f)
        k = mquery_conf['sketchSetup']['collPreliminarySelSize']

    with open(args.words_path) as f:
        words = f.readlines()

    if os.path.isfile(args.out_path):
        with open(args.out_path) as f:
            data = json.load(f)
        data[str(k)] = {}
    else:
        data = {str(k): {}}

    for i, word in enumerate(words):
        print(f'{100*i/len(words)}%')
        word = ' '.join(word.split())  # remove new lines and whitespaces
        results = {'modifiers-of': None, 'noun-modified-by': None, 'verbs-object': None, 'verbs-subject': None}
        for req_type in results:
            with urllib.request.urlopen(f'{args.server_path}/scoll/{args.corpname}/{req_type}?w={word}') as response:
                results[req_type] = json.loads(response.read())
        data[str(k)][word] = results
        with open(args.out_path, 'w') as f:
            json.dump(data, f)
    print('DONE')
