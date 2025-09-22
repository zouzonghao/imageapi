document.addEventListener('DOMContentLoaded', function () {
    // --- Element Cache ---
    const form = document.getElementById('generate-form');
    const submitBtn = document.getElementById('submit-btn');
    const loadingIndicator = document.getElementById('loading');
    const resultContainer = document.getElementById('result-container');
    const imageUploadGroup = document.getElementById('image-upload-group');
    const imageUrlGroup = document.querySelector('label[for="imageUrl"]').parentElement;
    const imageUpload = document.getElementById('image-upload');
    const imageUrlInput = document.getElementById('imageUrl');
    const previewContainer = document.getElementById('image-preview-container');
    const preview = document.getElementById('preview');
    const clearImageBtn = document.getElementById('clear-image-btn');
    const stepsInput = document.getElementById('steps');
    const stepsValue = document.getElementById('steps-value');
    const modelSelect = document.getElementById('model');
    const widthInput = document.getElementById('width');
    const heightInput = document.getElementById('height');
    const dynamicParams = document.querySelectorAll('.dynamic-param');
    const optimizeBtn = document.getElementById('optimize-btn');
    const promptTextarea = document.getElementById('prompt');
    const inputSizeLimitGroup = document.getElementById('input-size-limit-group');
   
    let modelsData = []; // To store the data from /api/models
   
    // --- 1. Dynamic Model & Parameter Loading ---
    fetch('/api/models')
        .then(response => response.json())
        .then(data => {
            modelsData = data;
            modelSelect.innerHTML = ''; // Clear existing options
            modelsData.forEach(provider => {
                const optgroup = document.createElement('optgroup');
                optgroup.label = provider.provider;
                provider.models.forEach(model => {
                    const option = document.createElement('option');
                    option.value = model.name;
                    option.textContent = model.name;
                    // Store the entire model info object in a data attribute
                    option.dataset.modelInfo = JSON.stringify(model);
                    optgroup.appendChild(option);
                   });
                   modelSelect.appendChild(optgroup);
            });
            // Trigger change event to set initial visibility
            modelSelect.dispatchEvent(new Event('change'));
        })
        .catch(error => {
            console.error('Error fetching models:', error);
            modelSelect.innerHTML = '<option value="">Error loading models</option>';
        });

    // --- 2. Update UI based on selected model ---
    modelSelect.addEventListener('change', function () {
        const selectedOption = this.options[this.selectedIndex];
        if (!selectedOption || !selectedOption.dataset.modelInfo) {
        	return;
        }
        const modelInfo = JSON.parse(selectedOption.dataset.modelInfo);
        const supportedParams = modelInfo.supported_params;
      
        // Toggle visibility of dynamic parameter controls
        dynamicParams.forEach(paramEl => {
            const paramName = paramEl.dataset.param;
            if (supportedParams.includes(paramName)) {
                paramEl.classList.remove('hidden');
            } else {
                paramEl.classList.add('hidden');
            }
        });

        // Update width and height input constraints
        if (modelInfo.max_width && modelInfo.max_height) {
        	widthInput.max = modelInfo.max_width;
        	widthInput.value = modelInfo.max_width;
        	heightInput.max = modelInfo.max_height;
        	heightInput.value = modelInfo.max_height;
        }
      
        // Toggle visibility of image-related inputs
        if (supportedParams.includes('image')) {
        	imageUploadGroup.classList.remove('hidden');
        	imageUrlGroup.classList.remove('hidden');
        	inputSizeLimitGroup.classList.remove('hidden');
        } else {
        	   imageUploadGroup.classList.add('hidden');
        	   imageUrlGroup.classList.add('hidden');
        	   inputSizeLimitGroup.classList.add('hidden');
        	   clearImage(); // Clear any selected image if the model doesn't support it
        }

        // Update steps slider constraints
        if (modelInfo.min_steps && modelInfo.max_steps && modelInfo.default_steps) {
            stepsInput.min = modelInfo.min_steps;
            stepsInput.max = modelInfo.max_steps;
            stepsInput.value = modelInfo.default_steps;
            stepsValue.textContent = modelInfo.default_steps;
        }
    });


    // --- 3. Handle Input Exclusivity ---
    imageUpload.addEventListener('change', function () {
        if (this.files && this.files[0]) {
            imageUrlInput.value = ''; // Clear URL input
            const reader = new FileReader();
            reader.onload = function (e) {
                preview.src = e.target.result;
                previewContainer.style.display = 'block';
                clearImageBtn.classList.remove('hidden');
            };
            reader.readAsDataURL(this.files[0]);
        }
    });

    imageUrlInput.addEventListener('input', function () {
        if (this.value) {
            clearFileInput(); // Clear file input if URL is typed
        }
    });

    function clearFileInput() {
        imageUpload.value = ''; // Clear the file input
        preview.src = '#';
        previewContainer.style.display = 'none';
        clearImageBtn.classList.add('hidden');
    }

    function clearImage() {
        clearFileInput();
        imageUrlInput.value = ''; // Clear the image URL input
    }

    clearImageBtn.addEventListener('click', clearImage);

    // --- 4. Form Submission ---
    form.addEventListener('submit', function (e) {
        e.preventDefault();

        const formData = new FormData(form);

        submitBtn.disabled = true;
        loadingIndicator.classList.remove('hidden');
        resultContainer.innerHTML = '<p>正在生成中，请稍候...</p>';

        fetch('/api/generate', {
            method: 'POST',
            body: formData
        })
            .then(response => {
                if (!response.ok) {
                    // Try to get error text from the server for better feedback
                    return response.text().then(text => { throw new Error(text || 'Server error') });
                }
                const contentType = response.headers.get("content-type");
                if (contentType && contentType.includes("application/json")) {
                    return response.json().then(data => ({ type: 'json', body: data }));
                } else if (contentType && contentType.startsWith("image/")) {
                    return response.blob().then(blob => ({ type: 'image', body: blob }));
                } else {
                    throw new Error('Unexpected response type from server.');
                }
            })
            .then(data => {
                if (data.type === 'json') {
                    resultContainer.innerHTML = `<img src="${data.body.imageUrl}" alt="Generated Image">`;
                } else if (data.type === 'image') {
                    const imageUrl = URL.createObjectURL(data.body);
                    resultContainer.innerHTML = `<img src="${imageUrl}" alt="Generated Image">`;
                }
            })
            .catch(error => {
                console.error('Error:', error);
                resultContainer.innerHTML = `<p class="error">生成失败: ${error.message}</p>`;
            })
            .finally(() => {
                submitBtn.disabled = false;
                loadingIndicator.classList.add('hidden');
            });
    });

    // --- 5. UI Helpers ---
    stepsInput.addEventListener('input', function () {
    	stepsValue.textContent = this.value;
    });
   
    optimizeBtn.addEventListener('click', function() {
    	const currentPrompt = promptTextarea.value;
    	if (!currentPrompt) {
    		alert('请输入提示词后再进行优化。');
    		return;
    	}
   
    	// Disable button and show loading state
    	this.disabled = true;
    	this.textContent = '正在优化...';
   
    	fetch('/api/optimize-prompt', {
    		method: 'POST',
    		headers: {
    			'Content-Type': 'application/json',
    		},
    		body: JSON.stringify({ prompt: currentPrompt }),
    	})
    	.then(response => {
    		if (!response.ok) {
    			throw new Error('优化失败，请稍后再试。');
    		}
    		return response.json();
    	})
    	.then(data => {
    		promptTextarea.value = data.optimized_prompt;
    	})
    	.catch(error => {
    		console.error('Error optimizing prompt:', error);
    		alert(error.message);
    	})
    	.finally(() => {
    		// Re-enable button and restore text
    		this.disabled = false;
    		this.textContent = '优化提示词';
    	});
    });
   });