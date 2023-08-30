import argparse
import json
import os

WORDS_PATH = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'words.txt'))

if __name__ == "__main__":
    argparser = argparse.ArgumentParser('Scoll analysis script')
    argparser.add_argument('--data', dest='data_paths', nargs='+', default=[], help='Data file paths')
    args = argparser.parse_args()

    # load and merge data from all files
    data = {}
    for data_path in args.data_paths:
        with open(data_path) as f:
            data.update(json.load(f))

    ks = sorted(data.keys(), key = lambda k: int(k))
    prev_k = ks[0]
    words = data[prev_k].keys()
    analysis = {}
    for k in ks[1:]:
        analysis[f'{prev_k}-{k}'] = {}
        new_words_count = {}

        # count entry difference between k for each word and query type
        for word in words:
            for query in data[prev_k][word]:
                prev_words = [v['word'] for v in data[prev_k][word][query]['freqs']]
                new_words = [v['word'] for v in data[k][word][query]['freqs'] if v['word'] not in prev_words]
                if query not in new_words_count:
                    new_words_count[query] = [len(new_words)]
                else:
                    new_words_count[query].append(len(new_words))

                if query in analysis[f'{prev_k}-{k}']:
                    analysis[f'{prev_k}-{k}'][query] += len(new_words)
                else:
                    analysis[f'{prev_k}-{k}'][query] = len(new_words)

        for query, counts in new_words_count.items():
            mean = sum(counts)/len(counts)
            dispersion = sum((mean-count)*(mean-count) for count in counts)/len(counts)
            analysis[f'{prev_k}-{k}'][query] = {
                "mean": mean,
                "dispersion": dispersion,
            }

        prev_k = k

    print(analysis)


