import argparse
import json
import os

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
            analysis[f'{prev_k}-{k}'][word] = {}
            for query in data[prev_k][word]:
                prev_words = [v['word'] for v in data[prev_k][word][query]['freqs']]
                new_words = [v['word'] for v in data[k][word][query]['freqs'] if v['word'] not in prev_words]
                analysis[f'{prev_k}-{k}'][word][query] = len(new_words)
        prev_k = k

    print(analysis)


