import requests
from bs4 import BeautifulSoup

def scrape_page(url):
    print(f"Fetching content from {url}...")
    try:
        response = requests.get(url, timeout=10)
        response.raise_for_status()
        
        soup = BeautifulSoup(response.text, 'html.parser')
        
        # Extract title and all h1 tags as a simple example
        title = soup.title.string if soup.title else "No title found"
        h1s = [h1.get_text() for h1 in soup.find_all('h1')]
        
        return {
            "title": title,
            "h1s": h1s
        }
    except Exception as e:
        return {"error": str(e)}

if __name__ == "__main__":
    target_url = "https://example.com"
    result = scrape_page(target_url)
    
    if "error" in result:
        print(f"Error: {result['error']}")
    else:
        print(f"Page Title: {result['title']}")
        print(f"H1 Headers: {result['h1s']}")
