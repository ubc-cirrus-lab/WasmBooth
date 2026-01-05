use hyper::{Body, Request, Response, Server};
use hyper::service::{make_service_fn, service_fn};
use serde::{Deserialize};
use std::convert::Infallible;
use std::net::SocketAddr;
use serde_json::Value;
use std::ptr;
use std::time::{Duration, Instant};
use base64::encode;
use rand_core::OsRng;
use serde::Serialize;
use image::{ImageBuffer, RgbImage};
use lopdf::content::{Content, Operation};
use lopdf::dictionary;
use lopdf::Dictionary;
use lopdf::Object::Name;
use lopdf::{Document, Object, Stream};
use rand::{distributions::Alphanumeric, Rng};
use std::io::Cursor;

#[derive(Debug, Deserialize)]
struct FuncInput {
    name: String,
    purchases: Vec<String>,
    price: Vec<f64>,
}

#[derive(Debug, Serialize)]
struct FuncResponse {
    resp: Vec<u8>,
}

#[inline(never)]
fn genpdf(event: FuncInput) -> FuncResponse {
    let name = event.name;
    let purchases: Vec<(&String, &f64)> = event.purchases.iter().zip(event.price.iter()).collect();

    let mut result: Vec<u8> = vec![];
    let mut doc = Document::with_version("1.5");
    let pages_id = doc.new_object_id();
    let font_id = doc.add_object(dictionary! {
        "Type" => "Font",
        "Subtype" => "Type1",
        "BaseFont" => "Courier",
    });
    let resources_id = doc.add_object(dictionary! {
        "Font" => dictionary! {
            "F1" => font_id,
        },
    });

    let mut pdf_ops = vec![
        Operation::new("BT", vec![]),
        Operation::new("Tf", vec!["F1".into(), 24.into()]),
        Operation::new("Td", vec![50.into(), 800.into()]),
        Operation::new(
            "Tj",
            vec![Object::string_literal(format!("Fake Bill for: {}", name))],
        ),
        Operation::new("ET", vec![]),
        Operation::new("BT", vec![]),
        Operation::new("Tf", vec!["F1".into(), 12.into()]),
        Operation::new("Td", vec![50.into(), 720.into()]),
        Operation::new(
            "Tj",
            vec![Object::string_literal(
                "-------------------------------------------------------------------",
            )],
        ),
        Operation::new("ET", vec![]),
        Operation::new("BT", vec![]),
        Operation::new("Tf", vec!["F1".into(), 12.into()]),
        Operation::new("Td", vec![50.into(), 700.into()]),
        Operation::new("Tj", vec![Object::string_literal("Purchases:")]),
        Operation::new("ET", vec![]),
    ];

    // Create multiple pages to store all the purchase
    let item_per_page = 50;
    let mut page_ids = Vec::new();
    let mut idx = 700 - 12;

    for chunk in purchases.chunks(item_per_page) {
        // Add the images
        let mut image_ops: Vec<Operation> = vec![];
        let mut dict = Dictionary::new();
        dict.set("Type", Object::Name(b"XObject".to_vec()));
        dict.set("Subtype", Object::Name(b"Image".to_vec()));
        dict.set("Width", 814);
        dict.set("Height", 613);
        dict.set("ColorSpace", Object::Name(b"DeviceRGB".to_vec()));
        dict.set("BitsPerComponent", 8);

        // For JPG files
        dict.set("Filter", Object::Name(b"DCTDecode".to_vec()));

        let img_stream = Stream::new(dict, random_images(814, 613));

        let img_position = (100.0, 210.0);
        let img_size = (100.0 + (814.0 / 3.0), 210.0 + (613.0 / 3.0));
        let img_id = doc.add_object(img_stream);
        let img_name = format!("X{}", img_id.0);
        image_ops.push(Operation::new("q", vec![]));
        image_ops.push(Operation::new(
            "cm",
            vec![
                img_size.0.into(),
                0.into(),
                0.into(),
                img_size.1.into(),
                img_position.0.into(),
                img_position.1.into(),
            ],
        ));
        image_ops.push(Operation::new(
            "Do",
            vec![Name(img_name.as_bytes().to_vec())],
        ));
        image_ops.push(Operation::new("Q", vec![]));
        image_ops.push(Operation::new("Q", vec![]));

        pdf_ops.extend(image_ops);

        // Add the items
        let mut purchase_ops: Vec<Operation> = vec![];
        for (purchase, price) in chunk.iter() {
            purchase_ops.push(Operation::new("BT", vec![]));
            purchase_ops.push(Operation::new("Tf", vec!["F1".into(), 12.into()]));
            purchase_ops.push(Operation::new("Td", vec![50.into(), idx.into()]));
            purchase_ops.push(Operation::new(
                "Tj",
                vec![Object::string_literal(format!(
                    "{}                                  ${:.2}",
                    purchase, price
                ))],
            ));
            purchase_ops.push(Operation::new("ET", vec![]));
            idx -= 12;
        }
        pdf_ops.extend(purchase_ops);

        // Save the changes to a page
        let content = Content {
            operations: pdf_ops,
        };

        let content_id = doc.add_object(Stream::new(dictionary! {}, content.encode().unwrap()));
        let page_id = doc.add_object(dictionary! {
            "Type" => "Page",
            "Parent" => pages_id,
            "Contents" => content_id,
        });

        doc.add_xobject(page_id, img_name.as_bytes(), img_id)
            .unwrap();

        page_ids.push(page_id);

        // Reset configurations
        idx = 700;
        // purchase_ops = vec![];
        pdf_ops = vec![];
    }

    let pages = dictionary! {
        "Type" => "Pages",
        "Kids" => page_ids.iter().map(|&id| id.into()).collect::<Vec<_>>(), // vec![page_id.into()],
        "Count" => page_ids.len() as i32,
        "Resources" => resources_id,
        "MediaBox" => vec![0.into(), 0.into(), 595.into(), 842.into()],
    };
    doc.objects.insert(pages_id, Object::Dictionary(pages));
    let catalog_id = doc.add_object(dictionary! {
        "Type" => "Catalog",
        "Pages" => pages_id,
    });
    doc.trailer.set("Root", catalog_id);
    doc.save_to(&mut result).unwrap();

    FuncResponse { resp: result }
}

fn random_images(w: u32, h: u32) -> Vec<u8> {
    let mut img: RgbImage = ImageBuffer::new(w, h);

    let mut rng = rand::thread_rng();
    for px in img.pixels_mut() {
        let r = rng.gen_range(200..=250);
        let g = rng.gen_range(200..=250);
        let b = rng.gen_range(200..=250);
        *px = image::Rgb([r, g, b]);
    }

    let mut result = Cursor::new(Vec::new());
    img.write_to(&mut result, image::ImageOutputFormat::Jpeg(80))
        .expect("Failed to write image");

    return result.into_inner();
}

async fn handle(req: Request<Body>) -> Result<Response<Body>, Infallible> {
    let start = Instant::now();
    
    // Only accept POST requests
    if req.method() != hyper::Method::POST {
        return Ok(Response::new(Body::from("Only POST method is supported")));
    }

    let mut purchases = Vec::new();
    let mut prices = Vec::new();

    for i in 1..=500 {
        // Genrate random item name
        let random_name = rand::thread_rng()
            .sample_iter(&Alphanumeric)
            .take(20) // 20 characters long
            .map(char::from)
            .collect::<String>();

        // You can increase this number for larger tests
        purchases.push(format!("{} item {}", i, random_name));
        prices.push((i as f64) * 1.23); // Example pricing formula
    }

    let filename = "test.pdf";
    let result: FuncResponse = genpdf(FuncInput {
        name: filename.to_string(),
        purchases: purchases,
        price: prices,
    });

    // Output the base64
    let pdf_base64 = base64::encode(&result.resp);

    let first_5_chars = &pdf_base64[..5];

    println!(
        "GENPDF Executed\n"
    );

    Ok(Response::new(Body::from(first_5_chars.to_string())))
}

#[tokio::main]
async fn _main() {
    let mut purchases = Vec::new();
    let mut prices = Vec::new();

    for i in 1..=500 {
        // Genrate random item name
        let random_name = rand::thread_rng()
            .sample_iter(&Alphanumeric)
            .take(20) // 20 characters long
            .map(char::from)
            .collect::<String>();

        // You can increase this number for larger tests
        purchases.push(format!("{} item {}", i, random_name));
        prices.push((i as f64) * 1.23); // Example pricing formula
    }

    let filename = "test.pdf";
    let result: FuncResponse = genpdf(FuncInput {
        name: filename.to_string(),
        purchases: purchases,
        price: prices,
    });

    // Output the base64
    let pdf_base64 = base64::encode(&result.resp);

    let first_5_chars = &pdf_base64[..5];

    println!(
        "GENPDF Executed\n"
    );
}

#[tokio::main]
async fn main() {
    let mut purchases = Vec::new();
    let mut prices = Vec::new();

    for i in 1..=500 {
        // Genrate random item name
        let random_name = rand::thread_rng()
            .sample_iter(&Alphanumeric)
            .take(20) // 20 characters long
            .map(char::from)
            .collect::<String>();

        // You can increase this number for larger tests
        purchases.push(format!("{} item {}", i, random_name));
        prices.push((i as f64) * 1.23); // Example pricing formula
    }

    let filename = "test.pdf";
    let result: FuncResponse = genpdf(FuncInput {
        name: filename.to_string(),
        purchases: purchases,
        price: prices,
    });

    // Output the base64
    let pdf_base64 = base64::encode(&result.resp);

    let first_5_chars = &pdf_base64[..5];

    println!(
        "GENPDF Executed\n"
    );
}
