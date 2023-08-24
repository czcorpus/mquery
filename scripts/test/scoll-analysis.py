import argparse
import json
import os
import difflib

WORDS_PATH = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'words.txt'))

if __name__ == "__main__":
    argparser = argparse.ArgumentParser('Scoll analysis script')
    argparser.add_argument('--data', dest='data_path', action='store', help='Data file path')
    args = argparser.parse_args()

    with open(args.data_path) as f:
        data = json.load(f)

    ks = sorted(data.keys(), key = lambda k: int(k))
    prev_k = ks[0]
    words = data[prev_k].keys()
    analysis = {}
    for k in ks[1:]:
        analysis[f'{prev_k}-{k}'] = {}
        for word in words:
            analysis[f'{prev_k}-{k}'][word] = len(list(difflib.ndiff(json.dumps(data[prev_k][word]), json.dumps(data[k][word]))))
        prev_k = k

    print(analysis)


