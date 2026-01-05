use hyper::{Body, Request, Response, Server};
use hyper::service::{make_service_fn, service_fn};
use serde::{Deserialize};
use std::convert::Infallible;
use std::net::SocketAddr;
use serde_json::Value;
use std::alloc::{alloc, dealloc, Layout};
use std::ptr;
use std::time::{Duration, Instant};
use rsa::PrivateKeyEncoding;
use rsa::{PublicKeyEncoding, RSAPrivateKey, RSAPublicKey};
use base64::encode;
use rand_core::OsRng;
use serde::Serialize;
use rand::{Rng, SeedableRng};
use rand::rngs::StdRng;
use compress::lz4::*;
use std::io::BufWriter;
use std::io::Write;

#[derive(Debug, Deserialize)]
struct FuncInput {
    tweets: Vec<String>,
}

#[derive(Debug, Serialize)]
struct FuncResponse {
    encoded_resp: Vec<String>,
}

#[inline(never)]
fn compress_input(data: Vec<u8>, mut encoder: Encoder<BufWriter<Vec<u8>>>) -> BufWriter<Vec<u8>> {
    encoder.write(&data).unwrap();
    let (compressed_bytes, _) = encoder.finish();
    return compressed_bytes;
}

#[inline(never)]
fn compress_json(event: FuncInput) -> FuncResponse {
    let mut resp = vec![];
    for tweet in event.tweets {
        let encoder = Encoder::new(BufWriter::new(Vec::new()));
        let compressed_bytes = compress_input(tweet.as_bytes().to_vec(), encoder);
        let encoded = encode(compressed_bytes.into_inner().unwrap());
        resp.push(encoded);
    }
    FuncResponse { encoded_resp: resp }
}

fn generate_large_tweets(count: usize, length: usize) -> Vec<String> {
    let seed: [u8; 32] = [42; 32]; // A constant seed (you can change this if needed)
    let mut rng: StdRng = SeedableRng::from_seed(seed);
    let mut tweets = Vec::with_capacity(count);

    for _ in 0..count {
        let tweet: String = (0..length)
            .map(|_| rng.sample(rand::distributions::Alphanumeric))
            .map(char::from)
            .collect();
        tweets.push(tweet);
    }

    tweets
}

async fn handle(req: Request<Body>) -> Result<Response<Body>, Infallible> {
    let start = Instant::now();
    
    // Only accept POST requests
    if req.method() != hyper::Method::POST {
        return Ok(Response::new(Body::from("Only POST method is supported")));
    }

    let tweet_count = 200000;
    let tweet_length = 100;
    
    let generated_tweets = generate_large_tweets(tweet_count, tweet_length);
    // println!("{:?}", generated_tweets.clone());

    let fun_res = compress_json(FuncInput {
        tweets: generated_tweets,
    });

    let first_five = &fun_res.encoded_resp[..5.min(fun_res.encoded_resp.len())];

    println!(
        "JSON-COMPRESSION Executed\n"
    );

    Ok(Response::new(Body::from("JSON-COMPRESSION Executed")))
}

#[tokio::main]
async fn _main() {
    let tweet_count = 200000;
    let tweet_length = 100;
    
    let generated_tweets = generate_large_tweets(tweet_count, tweet_length);
    // println!("{:?}", generated_tweets.clone());

    let fun_res = compress_json(FuncInput {
        tweets: generated_tweets,
    });

    let first_five = &fun_res.encoded_resp[..5.min(fun_res.encoded_resp.len())];

    println!(
        "JSON-COMPRESSION Executed\n"
    );
}

#[tokio::main]
async fn main() {
    let tweet_count = 200000;
    let tweet_length = 100;
    
    let generated_tweets = generate_large_tweets(tweet_count, tweet_length);
    // println!("{:?}", generated_tweets.clone());

    let fun_res = compress_json(FuncInput {
        tweets: generated_tweets,
    });

    let first_five = &fun_res.encoded_resp[..5.min(fun_res.encoded_resp.len())];

    println!(
        "JSON-COMPRESSION Executed\n"
    );
}
