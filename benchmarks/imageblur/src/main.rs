use hyper::{Body, Request, Response, Server};
use hyper::service::{make_service_fn, service_fn};
use std::convert::Infallible;
use std::net::SocketAddr;
// use std::alloc::{alloc, dealloc, Layout};
// use std::ptr;
use std::time::{Duration, Instant};
// use std::borrow::Cow;
// use base64::{encode, decode};
// use image::codecs::jpeg::JpegEncoder;
// use image::{load_from_memory_with_format, ImageFormat, ImageBuffer, Rgba, imageops};
// use serde::{Deserialize, Serialize};
// use std::io::Cursor;
// use rand::rngs::StdRng;
// use rand::{Rng, SeedableRng};

use std::borrow::Cow;
use base64::{encode, decode};
use image::codecs::jpeg::JpegEncoder;
use image::{load_from_memory_with_format, ImageFormat, ImageBuffer, Rgba, imageops};
use serde::{Deserialize, Serialize};
use std::io::Cursor;
use rand::rngs::StdRng;
use rand::{Rng, SeedableRng};


#[derive(Debug, Deserialize)]
struct FuncInput<'a> {
    image: Cow<'a, str>,
}

#[derive(Debug, Serialize)]
struct FuncResponse {
    image: String,
}

fn blur_inline(image: image::DynamicImage) -> ImageBuffer<Rgba<u8>, Vec<u8>> {
    imageops::blur(&image, 10.0)
}

fn generate_random_image(width: u32, height: u32) -> ImageBuffer<Rgba<u8>, Vec<u8>> {
    let seed: [u8; 32] = [42; 32];  // You can change the seed value here
    // Create a deterministic RNG using the seed
    let mut rng = StdRng::from_seed(seed);
    let mut img = ImageBuffer::new(width, height);

    for (_x, _y, pixel) in img.enumerate_pixels_mut() {
        let r = (rng.gen::<f32>() * 255.0) as u8;
        let g = (rng.gen::<f32>() * 255.0) as u8;
        let b = (rng.gen::<f32>() * 255.0) as u8;
        *pixel = Rgba([r, g, b, 255]); // RGBA format
    }

    img
}

#[inline(never)]
fn image_blur(event: FuncInput) -> FuncResponse {
    let image = decode(event.image.as_bytes()).unwrap();
    let decoded_image = load_from_memory_with_format(&image, ImageFormat::Jpeg).unwrap();

    let mut blurred = blur_inline(decoded_image);

    let mut output_buf = vec![];
    let mut jpeg_encoder = JpegEncoder::new(&mut output_buf);

    match jpeg_encoder.encode_image(&mut blurred) {
        Ok(_) => (),
        Err(err) => println!("Unable to encode image to PNG: {:?}", err),
    }
    /*
    let (nwidth, nheight) = blurred.dimensions();
    match jpeg_encoder.encode(&mut blurred.as_bytes(), nwidth, nheight, ColorType::Rgba8) {
        Ok(_) => (),
        Err(err) => println!("Unable to encode image to PNG: {:?}", err),
    }
    */
    FuncResponse { image: encode(output_buf) }
}

fn image_to_base64(image: ImageBuffer<Rgba<u8>, Vec<u8>>) -> String {
    let mut buffer = Vec::new();
    {
        // Save image to a buffer in JPEG format
        let mut cursor = Cursor::new(&mut buffer);
        image::DynamicImage::ImageRgba8(image)
            .write_to(&mut cursor, image::ImageOutputFormat::Jpeg(80)) // 80 is the quality (0-100)
            .unwrap();
    }
    // Encode the buffer as Base64
    encode(&buffer)
}

async fn handle(req: Request<Body>) -> Result<Response<Body>, Infallible> {
    let start = Instant::now();
    
    // Only accept POST requests
    if req.method() != hyper::Method::POST {
        return Ok(Response::new(Body::from("Only POST method is supported")));
    }

    let img = generate_random_image(1000, 1000);
    let b64_string = image_to_base64(img);
    
    // Call image_blur function
    let blur_image_input = FuncInput {
        image: Cow::from(b64_string.trim()),
    };
    let blur_image_res = image_blur(blur_image_input);
    let slice = &blur_image_res.image[0..5];

    println!(
        "IMAGEBLUR Executed\n"
    );

    Ok(Response::new(Body::from(slice.to_string())))
}

#[tokio::main]
async fn _main() {
    let img = generate_random_image(1000, 1000);
    let b64_string = image_to_base64(img);
    
    // Call image_blur function
    let blur_image_input = FuncInput {
        image: Cow::from(b64_string.trim()),
    };
    let blur_image_res = image_blur(blur_image_input);
    let slice = &blur_image_res.image[0..5];

    println!(
        "IMAGEBLUR Executed\n"
    );
}

#[tokio::main]
async fn main() {
    let img = generate_random_image(1000, 1000);
    let b64_string = image_to_base64(img);
    
    // Call image_blur function
    let blur_image_input = FuncInput {
        image: Cow::from(b64_string.trim()),
    };
    let blur_image_res = image_blur(blur_image_input);
    let slice = &blur_image_res.image[0..5];

    println!(
        "IMAGEBLUR Executed\n"
    );
}
